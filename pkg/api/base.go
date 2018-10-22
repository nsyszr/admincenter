package api

// ObjectResource contains a single resource
type ObjectResource struct {
	ResourceType string `json:"type"`
	URI          string `json:"uri"`
}

// CollectionResource contains a collection of resources
type CollectionResource struct {
	ObjectResource
	Members []interface{} `json:"members"`
	Total   int           `json:"total"`
}

type ConvertToResource func(interface{}) interface{}

func ToListResource(entities []interface{}, resourceType, uri string, toResource ConvertToResource) CollectionResource {
	var resource CollectionResource

	resource.ResourceType = resourceType
	resource.URI = uri

	for _, entity := range entities {
		resource.Members = append(resource.Members, toResource(entity))
	}

	resource.Total = len(resource.Members)

	return resource
}
