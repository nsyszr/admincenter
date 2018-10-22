package namespace

import "github.com/nsyszr/admincenter/pkg/api"

const (
	resourceTypeNamespace     = "NamespaceV100"
	resourceTypeNamespaceList = "NamespaceListV100"
)

type namespaceResource struct {
	api.ObjectResource
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func toResource(entity *Namespace) namespaceResource {
	var resource namespaceResource

	resource.ResourceType = resourceTypeNamespace
	resource.URI = "http://localhost:8080/api/namespaces/1234"
	resource.ID = entity.ID
	resource.Name = entity.Name

	return resource
}
