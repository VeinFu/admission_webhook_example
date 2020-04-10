package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/golang/glog"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()

	// (https://github.com/kubernetes/kubernetes/issues/57982)
	defaulter = runtime.ObjectDefaulter(runtimeScheme)
)

type patchType struct {
	Op string `json:"op"`
	Path string `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func main() {

	mux := http.NewServeMux()
	mux.HandleFunc("/validate", admissionHandler)
	mux.HandleFunc("/mutate", admissionHandler)

	httpServer := &http.Server{
		Addr: ":8999",
		Handler: mux,
	}

	if err := httpServer.ListenAndServeTLS("/etc/webhook/certs.d/server.crt", "/etc/webhook/certs.d/server.key"); err != nil {
		os.Exit(1)
	}
}

func admissionHandler(w http.ResponseWriter, r *http.Request) {

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		glog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		glog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		glog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		if r.URL.Path == "/validate" {
			admissionResponse = validateFunc(&ar)
		} else if r.URL.Path == "/mutate" {
			admissionResponse = mutateFunc(&ar)
		}
	}

	admissionReview := v1beta1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	glog.Infof("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func validateFunc(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request

	if req.Kind.Kind != "Pod" {
		glog.Infof("Skipping validation for %s/%s due to policy check", req.Namespace, req.Name)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object: %v", err)
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	podName, podNS, podMeta := pod.Name, pod.Namespace, pod.ObjectMeta
	availableLabels := pod.Labels

	fmt.Println(podName, podNS, podMeta)
	if podNS != "test-admisssion" {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	allow := true
	var result *metav1.Status
	requiredLabels := []string{"test-admission", "admission-webhook"}
	for _, key := range requiredLabels {
		if _, ok := availableLabels[key]; !ok {
			allow = false
			result = &metav1.Status{Reason: "required label is not set"}
		}
		break
	}

	return &v1beta1.AdmissionResponse{
		Allowed:          allow,
		Result:           result,
	}
}

func mutateFunc(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request

	if req.Kind.Kind != "Pod" {
		glog.Infof("Skipping validation for %s/%s due to policy check", req.Namespace, req.Name)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		glog.Errorf("Could not unmarshal raw object: %v", err)
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	podName, podNS, podMeta := pod.Name, pod.Namespace, pod.ObjectMeta
	availableLabels := pod.Labels

	fmt.Println(podName, podNS, podMeta)
	if podNS != "test-admisssion" {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	values := make(map[string]string)
	requiredLabels := []string{"test-admission", "admission-webhook"}
	for _, key := range requiredLabels {
		if _, ok := availableLabels[key]; !ok {
			values[key] = "yes"
		}
	}

	if len(values) == 0 {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	} else {
		var patchData []patchType

		// 因为admissionResponse仅仅支持jsonpatch更新策略，所以更新的字段或者
		// 内容应该是其对应的所有内容而不仅仅是新增的内容。
		for key, value := range availableLabels {
			values[key] = value
		}

		patchData = append(patchData, patchType{
			Op:    "add",
			Path:  "/metadata/labels",
			Value: values,
		})
		patchBytes, err := json.Marshal(patchData)
		if err != nil {
			glog.Errorf("Could not marshal patch data: %v", err)
			return &v1beta1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}

		glog.Infof("Patch data: %v", string(patchBytes))
		return &v1beta1.AdmissionResponse{
			Allowed:          true,
			Patch:            patchBytes,
			PatchType:        func() *v1beta1.PatchType {
				pt := v1beta1.PatchTypeJSONPatch
				return &pt
			}(),
		}
	}
}
