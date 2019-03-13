package svccontrol

import (
	"encoding/json"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	danmv1 "github.com/nokia/danm/crd/apis/danm/v1"
	"reflect"
)

const danmSelector = "danm.k8s.io/selector"
const danmNetwork = "danm.k8s.io/network"
const TolerateUnreadyEps = "service.alpha.kubernetes.io/tolerate-unready-endpoints"

func IsContain(ep, svc map[string]string) bool {
	epFit := true
	for k, v := range svc {
		if val, ok := ep[k]; ok {
			if val != v {
				epFit = false
				break
			}
		} else {
			epFit = false
			break
		}
	}
	return epFit
}

func GetDanmSvcAnnotations(annotations map[string]string) (map[string]string, string, error) {
	selectorMap := make(map[string]string)
	svcNet := ""
	if danmSel, ok := annotations[danmSelector]; ok {
		if danmSel != "" {
			err := json.Unmarshal([]byte(danmSel), &selectorMap)
			if err != nil {
				glog.Errorf("utils: json error: %s", err)
				return selectorMap, svcNet, err
			}
		}
	}

	if danmNet, ok := annotations[danmNetwork]; ok {
		if danmNet != "" {
			svcNet = danmNet
		}
	}

	return selectorMap, svcNet, nil
}

func PodReady(pod *corev1.Pod) bool {
	for i := range pod.Status.Conditions {
		if pod.Status.Conditions[i].Type == corev1.PodReady && pod.Status.Conditions[i].Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func SelectDesMatchLabels(des []*danmv1.DanmEp, selectorMap map[string]string, svcNet string, svcNs string) []*danmv1.DanmEp {
	var deList []*danmv1.DanmEp
	for _, de := range des {
		deFit := true
		if de.GetNamespace() != svcNs {
			deFit = false
		} else {
			deMap := de.GetLabels()
			deFit = IsContain(deMap, selectorMap)
			if deFit && de.Spec.NetworkID != svcNet {
				deFit = false
			}
		}
		if deFit {
			deList = append(deList, de.DeepCopy())
		}
	}
	return deList
}

func FindEpsForSvc(eps []*corev1.Endpoints, svcName, svcNs string) bool {
	epFound := false
	for _, ep := range eps {
		epName := ep.GetName()
		if epName == svcName && ep.GetNamespace() == svcNs {
			epFound = true
			break
		}
	}
	return epFound
}

func SvcChanged(oldSvc, newSvc *corev1.Service) bool {
	// danm svc annotations and annotations.ealy change are relevant
	oldAnno := oldSvc.Annotations
	newAnno := newSvc.Annotations
	oldSelMap, oldNet, oldErr := GetDanmSvcAnnotations(oldAnno)
	newSelMap, newNet, newErr := GetDanmSvcAnnotations(newAnno)
	if oldErr != nil || newErr != nil {
		return true
	}
	if reflect.DeepEqual(oldSelMap, newSelMap) && oldNet == newNet && reflect.DeepEqual(oldSvc.Spec.Ports, newSvc.Spec.Ports) && (oldSvc.Annotations[TolerateUnreadyEps] == newSvc.Annotations[TolerateUnreadyEps]) {
		// no change
		return false
	}
	return true
}

func PodLabelChanged(oldPod, newPod *corev1.Pod) bool {
	if reflect.DeepEqual(oldPod.GetLabels(), newPod.GetLabels()) {
		// no change
		return false
	}
	return true
}

func MatchExistingSvc(de *danmv1.DanmEp, servicesList []*corev1.Service) []*corev1.Service {
	deNs := de.Namespace
	var svcList []*corev1.Service
	for _, svc := range servicesList {
		annotations := svc.GetAnnotations()
		selectorMap, svcNet, err := GetDanmSvcAnnotations(annotations)
		if err != nil {
			return svcList
		}
		if len(selectorMap) == 0 || svcNet != de.Spec.NetworkID || svc.GetNamespace() != deNs {
			continue
		}
		deMap := de.GetLabels()
		deFit := IsContain(deMap, selectorMap)
		if !deFit {
			continue
		}
		svcList = append(svcList, svc.DeepCopy())
	}
	return svcList
}

