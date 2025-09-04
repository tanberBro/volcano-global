package util

import (
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
)

func Errored(code int32, err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Code:    code,
			Message: err.Error(),
		},
	}
}

// Allowed constructs a response indicating that the given operation
// is allowed (without any patches).
func Allowed(message string) *admissionv1.AdmissionResponse {
	return ValidationResponse(true, message)
}

// Denied constructs a response indicating that the given operation
// is not allowed.
func Denied(message string) *admissionv1.AdmissionResponse {
	return ValidationResponse(false, message)
}

// ValidationResponse returns a response for admitting a request.
func ValidationResponse(allowed bool, message string) *admissionv1.AdmissionResponse {
	code := http.StatusForbidden
	reason := metav1.StatusReasonForbidden
	if allowed {
		code = http.StatusOK
		reason = ""
	}
	resp := &admissionv1.AdmissionResponse{
		Allowed: allowed,
		Result: &metav1.Status{
			Code:   int32(code),
			Reason: reason,
		},
	}
	if len(message) > 0 {
		resp.Result.Message = message
	}
	return resp
}
