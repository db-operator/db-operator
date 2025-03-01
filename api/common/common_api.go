package common

// From ref should be used to get data from a ConfigMap or Secret
type FromRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}
