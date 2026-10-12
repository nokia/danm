package ipam

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"reflect"
	"strconv"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"
	danmtypes "github.com/danm-cni/danm/crd/apis/danm/v1"
	danmclientset "github.com/danm-cni/danm/crd/client/clientset/versioned"
	"github.com/danm-cni/danm/pkg/bitarray"
	"github.com/danm-cni/danm/pkg/datastructs"
	"github.com/danm-cni/danm/pkg/netcontrol"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	NoneAllocType       = "none"
	DynamicAllocType    = "dynamic"
	WithPrefix          = true
	V4Suffix            = "V4"
	V6Suffix            = "V6"
	UnshareableStaticIp = false
)

type ripRecord struct {
	shareable  bool
	ownerName  string
	ownerNs    string
	ripAddress string
}

type reservedIps map[string]ripRecord

// Reserve inspects the network object received as an input, and allocates an IPv4 or IPv6 address from the appropriate allocation pool
// In case static IP allocation is requested, it will try to reserve the requested address. If it is not possible, it returns an error
// Static IP can be also request via providing a reference to a ReservedIP object holding the desired address. Allocations done via this feature may be shared between multiple Pods
// The allocated IP addresses are represented by setting a bit in the network's BitArray type allocation matrices
// The refreshed network object is modified in the K8s API server at the end
func Reserve(danmClient danmclientset.Interface, netInfo danmtypes.DanmNet, req4, req6, requestNamespace string) (datastructs.IpamResult, error) {
	origSpec := netInfo.Spec
	tempNet := netInfo
	for {
		ipamResult, err := allocateIps(danmClient, &tempNet, req4, req6, requestNamespace)
		if err != nil {
			return datastructs.IpamResult{}, errors.New("failed to allocate IP address for network:" + netInfo.ObjectMeta.Name + " with error:" + err.Error())
		}
		//There is nothing to update in the API server if the network is unchanged after IP reservation
		if reflect.DeepEqual(origSpec, tempNet.Spec) {
			return ipamResult, nil
		}
		retryNeeded, newNet, err := updateIpAllocation(danmClient, tempNet)
		if err != nil {
			return datastructs.IpamResult{}, err
		}
		if retryNeeded {
			tempNet = newNet
			origSpec = newNet.Spec
			continue
		}
		return ipamResult, nil
	}
}

// freeIp inspects the network object received as an input, and releases an IPv4 or IPv6 address from the appropriate allocation pool
// The IP address liberation is represented by unsetting a bit in the network's BitArray type allocation matrix
func freeIp(netInfo *danmtypes.DanmNet, rip string) {
	if rip == NoneAllocType || rip == "" {
		return
	}
	ripParts := strings.Split(rip, "/")
	ip := net.ParseIP(ripParts[0])

	if ip.To4() != nil {
		netInfo.Spec.Options.Alloc = resetIp(netInfo.Spec.Options.Alloc, netInfo.Spec.Options.Cidr, ip)
	} else {
		netInfo.Spec.Options.Alloc6 = resetIp(netInfo.Spec.Options.Alloc6, netInfo.Spec.Options.Pool6.Cidr, ip)
	}
}

func allocateIps(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, req4, req6, requestNamespace string) (datastructs.IpamResult, error) {
	var ip4, ip6, rip4, rip6 string
	reservedIps, err := getReservedIps(danmClient, netInfo, req4, req6, requestNamespace)
	if err != nil {
		return datastructs.IpamResult{}, err
	}
	if req4 != "" {
		netInfo.Spec.Options.Alloc, ip4, rip4, err = allocateAddress(&netInfo.Spec.Options.Pool, netInfo.Spec.Options.Alloc, req4, netInfo.Spec.Options.Cidr, netInfo.Spec.Options.Cidr, netInfo.Spec.Options.Routes, requestNamespace, reservedIps, V4Suffix)
		if err != nil {
			return datastructs.IpamResult{}, err
		}
	}
	if req6 != "" {
		if netInfo.Spec.Options.Net6 != "" && netInfo.Spec.Options.Pool6.Cidr == "" {
			InitV6AllocFields(netInfo)
		}
		//TODO: to have a real uniform handling both V4 and V6 pool definition should be uniform, meaning, V4 pools should also have a separare allocation CIDR
		tempPool6 := danmtypes.IpPool{Start: netInfo.Spec.Options.Pool6.Start, End: netInfo.Spec.Options.Pool6.End, LastIp: netInfo.Spec.Options.Pool6.LastIp}
		netInfo.Spec.Options.Alloc6, ip6, rip6, err = allocateAddress(&tempPool6, netInfo.Spec.Options.Alloc6, req6, netInfo.Spec.Options.Pool6.Cidr, netInfo.Spec.Options.Net6, netInfo.Spec.Options.Routes6, requestNamespace, reservedIps, V6Suffix)
		if err != nil {
			return datastructs.IpamResult{}, err
		}
		netInfo.Spec.Options.Pool6.LastIp = tempPool6.LastIp
	}
	ipamResult := datastructs.IpamResult{Ip4: ip4, Ip6: ip6, Rip4: rip4, Rip6: rip6}
	return ipamResult, err
}

func InitV6AllocFields(netInfo *danmtypes.DanmNet) {
	InitV6PoolCidr(netInfo)
	netInfo.Spec.Options.Pool6.Start, netInfo.Spec.Options.Pool6.End, netInfo.Spec.Options.Alloc6 =
		InitAllocPool(netInfo.Spec.Options.Pool6.Cidr, netInfo.Spec.Options.Pool6.Start, netInfo.Spec.Options.Pool6.End, netInfo.Spec.Options.Alloc6, netInfo.Spec.Options.Routes6)
}

func allocateAddress(pool *danmtypes.IpPool, alloc, reqType, allocCidr, netCidr string, routes map[string]string, requestNamespace string, reservedIps reservedIps, ripSuffix string) (string, string, string, error) {
	if reqType == NoneAllocType {
		return alloc, NoneAllocType, "", nil
	}
	if alloc == "" {
		return alloc, "", "", errors.New("IP address cannot be allocated for an L2 network!")
	}
	ba := bitarray.NewBitArrayFromBase64(alloc)
	var allocatedIp string
	_, allocSubnet, _ := net.ParseCIDR(allocCidr)
	_, netSubnet, _ := net.ParseCIDR(netCidr)
	if reqType == DynamicAllocType {
		begin, end := getAllocRangeBasedOnCidr(pool, allocSubnet)
		var lastIpIndex uint32
		if pool.LastIp != "" {
			//Support both CIDR and plain IP format just in-case
			lastIp, _, _ := net.ParseCIDR(pool.LastIp)
			if lastIp == nil {
				lastIp = net.ParseIP(pool.LastIp)
			}
			lastIpIndex = GetIndexOfIp(lastIp, allocSubnet) + 1
		}
		if lastIpIndex > end || lastIpIndex < begin {
			lastIpIndex = begin
		}
		var doesAnyFreeIpExist bool
		var allocatedIndex uint32
		for i := lastIpIndex; i <= end; i++ {
			if !ba.Get(i) && !isIpAGwIp(i, routes, ba, allocSubnet) && !isIpReserved(getIpFromIndex(i, allocSubnet, netSubnet, !WithPrefix), reservedIps) {
				ba.Set(i)
				allocatedIndex = i
				doesAnyFreeIpExist = true
				break
			}
			//Now let's look from the beginning until LastIp
			if i == end && end != lastIpIndex {
				i = begin - 1
				end = lastIpIndex
			}
		}
		if !doesAnyFreeIpExist {
			return alloc, "", "", errors.New("IP address cannot be dynamically allocated, all addresses are reserved!")
		}
		allocatedIp = getIpFromIndex(allocatedIndex, allocSubnet, netSubnet, WithPrefix)
		pool.LastIp = allocatedIp
		return ba.Encode(), allocatedIp, "", nil
	}
	//I guess we are doing backward compatibility now :)
	//You used to be able to define a static IP in CIDR format, so now we need to trim the suffix if it is still unnecessarily there
	requestParts := strings.Split(reqType, "/")
	ip := net.ParseIP(requestParts[0])
	var err error
	var ripRef string
	//Old-school static IP allocation
	if ip != nil {
		if isIpReserved(requestParts[0], reservedIps) {
			return alloc, "", "", errors.New("static IP allocation failed, requested static IP:" + requestParts[0] + " is reserved through the ReservedIP API")
		}
		allocatedIp, err = allocateStaticIp(ip, netSubnet, allocSubnet, ba, UnshareableStaticIp)
		if err != nil {
			return alloc, "", "", err
		}
	} else {
		//ReservedIP reference style allocation... or malformed static IP request. no way to differentiate
		requestedReservedIpRecord, exists := reservedIps[reqType+"/"+requestNamespace+"/"+ripSuffix]
		if !exists {
			return alloc, "", "", errors.New("static IP allocation failed, requested static IP:" + reqType + " is either a malformed IP address, or a non-existent ReservedIP reference")
		}
		//Only storing RIP reference in DanmEp for Shareable allocations means for the majority of the cases we do not need to do an extra lookup during a delete
		if requestedReservedIpRecord.shareable {
			ripRef = reqType
		}
		requestedReservedIp := net.ParseIP(requestedReservedIpRecord.ripAddress)
		allocatedIp, err = allocateStaticIp(requestedReservedIp, netSubnet, allocSubnet, ba, requestedReservedIpRecord.shareable)
		if err != nil {
			return alloc, "", "", err
		}
	}
	return ba.Encode(), allocatedIp, ripRef, nil
}

func getAllocRangeBasedOnCidr(pool *danmtypes.IpPool, cidr *net.IPNet) (uint32, uint32) {
	var beginAsInt, endAsInt uint32
	if cidr.IP.To4() != nil {
		firstIpAsInt := Ip2int(cidr.IP)
		beginAsInt = Ip2int(net.ParseIP(pool.Start)) - firstIpAsInt
		endAsInt = Ip2int(net.ParseIP(pool.End)) - firstIpAsInt
	} else {
		firstIpAsBigInt := Ip62int(cidr.IP)
		beginAsBigInt := Ip62int(net.ParseIP(pool.Start))
		endAsBigInt := Ip62int(net.ParseIP(pool.End))
		beginAsInt = uint32(beginAsBigInt.Sub(beginAsBigInt, firstIpAsBigInt).Uint64())
		endAsInt = uint32(endAsBigInt.Sub(endAsBigInt, firstIpAsBigInt).Uint64())
	}
	return beginAsInt, endAsInt
}

func GetIndexOfIp(ip net.IP, subnet *net.IPNet) uint32 {
	var index uint32
	if ip == nil {
		return index
	}
	if ip.To4() != nil {
		firstIpAsInt := Ip2int(subnet.IP)
		ipAsInt := Ip2int(ip)
		index = ipAsInt - firstIpAsInt
	} else {
		firstIpAsBigInt := Ip62int(subnet.IP)
		ipAsBigInt := Ip62int(ip)
		index = uint32(ipAsBigInt.Sub(ipAsBigInt, firstIpAsBigInt).Uint64())
	}
	return index
}

func getIpFromIndex(index uint32, allocSubnet, netSubnet *net.IPNet, withPrefix bool) string {
	var ip net.IP
	if allocSubnet.IP.To4() != nil {
		firstIpAsInt := Ip2int(allocSubnet.IP)
		ip = Int2ip(firstIpAsInt + index)
	} else {
		firstIpAsBigInt := Ip62int(allocSubnet.IP)
		indexAsBigInt := new(big.Int).SetUint64(uint64(index))
		ip = Int2ip6(firstIpAsBigInt.Add(firstIpAsBigInt, indexAsBigInt))
	}
	prefix, _ := netSubnet.Mask.Size()
	if withPrefix {
		return ip.String() + "/" + strconv.Itoa(prefix)
	}
	return ip.String()
}

func updateIpAllocation(danmClient danmclientset.Interface, netInfo danmtypes.DanmNet) (bool, danmtypes.DanmNet, error) {
	resourceConflicted, err := netcontrol.PutNetwork(danmClient, &netInfo)
	if err != nil {
		return false, danmtypes.DanmNet{}, errors.New("DanmNet update failed with error:" + err.Error())
	}
	if resourceConflicted {
		newNetSpec, err := netcontrol.RefreshNetwork(danmClient, netInfo)
		if err != nil {
			return false, danmtypes.DanmNet{}, errors.New("After IP address reservation conflict, network cannot be read again!")
		}
		return true, *newNetSpec, nil
	}
	return false, danmtypes.DanmNet{}, nil
}

func resetIp(alloc, cidr string, rip net.IP) string {
	ba := bitarray.NewBitArrayFromBase64(alloc)
	_, subnet, _ := net.ParseCIDR(cidr)
	if rip == nil || subnet == nil || !subnet.Contains(rip) {
		//Invalid IP, nothing to do here. Resetting would crash if we wouldn't return
		return alloc
	}
	allocatedIndex := GetIndexOfIp(rip, subnet)
	ba.Reset(allocatedIndex)
	return ba.Encode()
}

func GarbageCollectIps(danmClient danmclientset.Interface, netInfo danmtypes.DanmNet, ip4, ip6 string) error {
	tempNet := netInfo
	origSpec := netInfo.Spec
	for {
		freeIp(&tempNet, ip4)
		freeIp(&tempNet, ip6)
		if reflect.DeepEqual(origSpec, tempNet.Spec) {
			return nil
		}
		retryNeeded, newNet, err := updateIpAllocation(danmClient, tempNet)
		if err != nil {
			return err
		}
		if retryNeeded {
			tempNet = newNet
			origSpec = newNet.Spec
			continue
		}
		return nil
	}
}

// Ip2int converts an IP address stored according to the Golang net package to a native Golang big endian, 32-bit integer
func Ip2int(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}

// Ip62int converts an IPv6 address stored according to the Golang net package to a native Golang big endian, 64-bit integer
func Ip62int(ip6 net.IP) *big.Int {
	Ip6Int := big.NewInt(0)
	Ip6Int.SetBytes(ip6.To16())
	return Ip6Int
}

// Int2ip converts an IP address stored as a native Golang big endian, 32-bit integer to an IP
// represented according to the Golang net package
func Int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}

// Int2ip6 converts an IP address stored as a native Golang big endian, 64-bit integer to an IP
// represented according to the Golang net package
func Int2ip6(nn *big.Int) net.IP {
	ip := nn.Bytes()
	return ip
}

func CreateAllocationArray(subnet *net.IPNet, routes map[string]string) string {
	bitArray, _ := bitarray.CreateBitArrayFromIpnet(subnet)
	return bitArray.Encode()
}

func isIpAGwIp(ipPosition uint32, routes map[string]string, bitArray *bitarray.BitArray, subnet *net.IPNet) bool {
	for _, gw := range routes {
		gatewayPosition := GetIndexOfIp(net.ParseIP(gw), subnet)
		if gatewayPosition <= bitArray.Len() && gatewayPosition == ipPosition {
			return true
		}
	}
	return false
}

func DoV6CidrsIntersect(masterCidr, subCidr *net.IPNet) bool {
	firstAllocIp, lastAllocIp := cidr.AddressRange(subCidr)
	//Brute force: if the Alloc6 CIDR's first, and last IP both belongs to Net6, we assume the whole CIDR also does
	if masterCidr.Contains(firstAllocIp) && masterCidr.Contains(lastAllocIp) {
		return true
	}
	return false
}

func GetMaxUsableV6Prefix(dnet *danmtypes.DanmNet) int {
	if dnet.Spec.Options.Cidr == "" {
		return datastructs.MaxV6PrefixLength
	}
	_, v4Cidr, _ := net.ParseCIDR(dnet.Spec.Options.Cidr)
	sizeOfV4AllocPool := cidr.AddressCount(v4Cidr)
	maxRemainingCapacity := uint64(math.Pow(2, float64(bitarray.MaxSupportedAllocLength))) - sizeOfV4AllocPool
	maxUsableV6Prefix := datastructs.MinV6PrefixLength
	for pref := datastructs.MaxV6PrefixLength; pref <= datastructs.MinV6PrefixLength; pref++ {
		_, testCidr, _ := net.ParseCIDR("2a00:8a00:a000:1193:f816:3eff:fe24:e348/" + strconv.Itoa(pref))
		if cidr.AddressCount(testCidr) < maxRemainingCapacity {
			maxUsableV6Prefix = pref
			break
		}
	}
	return maxUsableV6Prefix
}

func InitV6PoolCidr(netInfo *danmtypes.DanmNet) {
	if netInfo.Spec.Options.Net6 == "" || netInfo.Spec.Options.Pool6.Cidr != "" {
		return
	}
	_, netCidr, _ := net.ParseCIDR(netInfo.Spec.Options.Net6)
	baseCidrStart := netCidr.IP
	pool6CidrPrefix := GetMaxUsableV6Prefix(netInfo)
	//If the subnet of the whole network is smaller than the maximum remaining capacity, use only that amount
	net6Prefix, _ := netCidr.Mask.Size()
	if pool6CidrPrefix < net6Prefix {
		pool6CidrPrefix = net6Prefix
	}
	maskedV6AllocCidrBase := net.CIDRMask(pool6CidrPrefix, 128)
	maskedV6AllocCidr := net.IPNet{IP: baseCidrStart, Mask: maskedV6AllocCidrBase}
	netInfo.Spec.Options.Pool6.Cidr = maskedV6AllocCidr.String()
}

func InitAllocPool(netCidr, start, end, alloc string, routes map[string]string) (string, string, string) {
	if netCidr == "" {
		return start, end, alloc
	}
	_, allocCidr, _ := net.ParseCIDR(netCidr)
	if start == "" {
		start = cidr.Inc(allocCidr.IP).String()
	}
	if end == "" {
		end = cidr.Dec(GetBroadcastAddress(allocCidr)).String()
	}
	if alloc == "" {
		alloc = CreateAllocationArray(allocCidr, routes)
	}
	return start, end, alloc
}

func GetBroadcastAddress(subnet *net.IPNet) net.IP {
	_, lastIp := cidr.AddressRange(subnet)
	return lastIp
}

func WasIpAllocatedByDanm(ip string, netInfo *danmtypes.DanmNet) bool {
	if netInfo == nil {
		return false
	}
	//Interface without IP is a DANM special so DanmEps having such attributes need to be treated as "DANM-managed" to ensure they are properly cleaned
	//IP freeing is already prepared to handle these types
	if ip == NoneAllocType || ip == "" {
		return true
	}

	parsedIp := net.ParseIP(ip)
	if parsedIp == nil {
		parsedIp, _, _ = net.ParseCIDR(ip)
	}
	//Static IP allocations can fall outside of the allocation pool but must be inside the network's CIDR so such an IP is still "DANM allocated" even though no bit is reserved for it in Alloc/Alloc6
	//ipam.resetIP can handle them gracefully
	var netCidr *net.IPNet
	if parsedIp.To4() != nil {
		_, netCidr, _ = net.ParseCIDR(netInfo.Spec.Options.Cidr)
	} else {
		_, netCidr, _ = net.ParseCIDR(netInfo.Spec.Options.Net6)
	}
	if parsedIp != nil && netCidr != nil && netCidr.Contains(parsedIp) {
		return true
	}
	return false
}

func getReservedIps(danmClient danmclientset.Interface, netInfo *danmtypes.DanmNet, req4, req6, requestNamespace string) (reservedIps, error) {
	reservedIps := make(reservedIps)
	//Build up the reservedIps cache but only if an allocation is actually expected to take place
	if ((req4 == "" || req4 == NoneAllocType) && (req6 == "" || req6 == NoneAllocType)) || netInfo.Kind == netcontrol.DanmNetKind || netInfo.Kind == "" {
		return reservedIps, nil
	}
	for _, statusRip := range netInfo.Status.ReservedIPs {
		labelSelectorListOpt := meta_v1.ListOptions{LabelSelector: labels.Set{
			datastructs.NetworkNameLabel: netInfo.Name,
			datastructs.NetworkTypeLabel: netInfo.Kind,
		}.String()}
		rips, err := danmClient.DanmV1().ReservedIPs(statusRip.Namespace).List(context.TODO(), labelSelectorListOpt)
		if err != nil {
			return reservedIps, fmt.Errorf("failed to get ReservedIP objects from the API Server because: %w", err)
		}
		for _, rip := range rips.Items {
			for _, ip := range rip.Spec.Ips {
				ripRecord := ripRecord{ownerName: rip.Name, ownerNs: rip.Namespace, shareable: ip.Shareable, ripAddress: ip.Address}
				reservedIps[ip.Address] = ripRecord
				//KA-CHING: at least one of requested IP allocations will ask for the lookup of this exact ReservedIP object
				//It looks kinda stupid but if we create an additional "reverse link" record to this object we can immediately access the required values later without either doing another lookup, or maintaining two separate lists in memory
				if rip.Namespace == requestNamespace && (req4 == rip.Name || req6 == rip.Name) {
					var suffix string
					netIp := net.ParseIP(ip.Address)
					if netIp.To4() != nil {
						suffix = V4Suffix
					} else {
						suffix = V6Suffix
					}
					reservedIps[rip.Name+"/"+rip.Namespace+"/"+suffix] = ripRecord
				}
			}
		}
	}
	return reservedIps, nil
}

func isIpReserved(ip string, reservedIps reservedIps) bool {
	_, exists := reservedIps[ip]
	return exists
}

func allocateStaticIp(ip net.IP, netSubnet, allocSubnet *net.IPNet, ba *bitarray.BitArray, isShareable bool) (string, error) {
	if !(netSubnet.Contains(ip)) {
		return "", errors.New("static IP allocation failed, requested static IP:" + ip.String() + " is outside the network's CIDR:" + netSubnet.String())
	}
	//Static IP allocations can come from the network's CIDR, outside of the dynamic allocation pool
	//But if the static IP does belong to the allocation pool, we need to reserve its place as usual
	if allocSubnet.Contains(ip) {
		allocatedIndex := GetIndexOfIp(ip, allocSubnet)
		//TODO: we should throw the same error if static IP is outside the allocation pool, but was already assigned to a DanmEp
		if ba.Get(allocatedIndex) && !isShareable {
			return "", errors.New("static IP allocation failed, requested IP address:" + ip.String() + " is already in use")
		}
		ba.Set(allocatedIndex)
	}
	prefix, _ := netSubnet.Mask.Size()
	return ip.String() + "/" + strconv.Itoa(prefix), nil
}
