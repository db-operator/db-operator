package v1

// NamespacedName is a fork of the kubernetes api type of the same name.
// Sadly this is required because CRD structs must have all fields json tagged and the kubernetes type is not tagged.
type NamespacedName struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}
