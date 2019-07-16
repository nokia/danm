package http

import (
  "bytes"
  "errors"
  "io/ioutil"
  "net/http"
  "k8s.io/api/admission/v1beta1"
  "k8s.io/apimachinery/pkg/runtime"
  "k8s.io/apimachinery/pkg/runtime/serializer"
)

type ResponseWriterStub struct {
  RespHeader http.Header
  Response []byte
}

func NewWriterStub() *ResponseWriterStub {
  header := make(map[string][]string,5)
  writer := ResponseWriterStub{RespHeader: header}
  return &writer
}

func (writer *ResponseWriterStub) Header() http.Header {
  return writer.RespHeader
}

func (writer *ResponseWriterStub) Write(response []byte) (int, error) {
  writer.Response = response
  return 200, nil
}

func (writer *ResponseWriterStub) WriteHeader(statusCode int) {
  return
}

func (writer *ResponseWriterStub) GetAdmissionResponse() (*v1beta1.AdmissionResponse,error) {
  if writer.Response == nil {
    return nil, errors.New("no response was sent")
  }
  review := v1beta1.AdmissionReview{}
  reader := bytes.NewReader(writer.Response)
  readCloser := ioutil.NopCloser(reader)
  payload, err := ioutil.ReadAll(readCloser)
  if err != nil {
    return nil, err
  }
  codecs := serializer.NewCodecFactory(runtime.NewScheme())
  deserializer := codecs.UniversalDeserializer()
  _, _, err = deserializer.Decode(payload, nil, &review)
  return review.Response, err
}