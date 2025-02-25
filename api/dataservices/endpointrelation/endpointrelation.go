package endpointrelation

import (
	"fmt"

	portainer "github.com/portainer/portainer/api"
	"github.com/portainer/portainer/api/internal/edge/cache"

	"github.com/rs/zerolog/log"
)

const (
	// BucketName represents the name of the bucket where this service stores data.
	BucketName = "endpoint_relations"
)

// Service represents a service for managing environment(endpoint) relation data.
type Service struct {
	connection    portainer.Connection
	updateStackFn func(ID portainer.EdgeStackID, updateFunc func(edgeStack *portainer.EdgeStack)) error
}

func (service *Service) BucketName() string {
	return BucketName
}

func (service *Service) RegisterUpdateStackFunction(updateFunc func(ID portainer.EdgeStackID, updateFunc func(edgeStack *portainer.EdgeStack)) error) {
	service.updateStackFn = updateFunc
}

// NewService creates a new instance of a service.
func NewService(connection portainer.Connection) (*Service, error) {
	err := connection.SetServiceName(BucketName)
	if err != nil {
		return nil, err
	}

	return &Service{
		connection: connection,
	}, nil
}

// EndpointRelations returns an array of all EndpointRelations
func (service *Service) EndpointRelations() ([]portainer.EndpointRelation, error) {
	var all = make([]portainer.EndpointRelation, 0)

	err := service.connection.GetAll(
		BucketName,
		&portainer.EndpointRelation{},
		func(obj interface{}) (interface{}, error) {
			r, ok := obj.(*portainer.EndpointRelation)
			if !ok {
				log.Debug().Str("obj", fmt.Sprintf("%#v", obj)).Msg("failed to convert to EndpointRelation object")
				return nil, fmt.Errorf("Failed to convert to EndpointRelation object: %s", obj)
			}

			all = append(all, *r)

			return &portainer.EndpointRelation{}, nil
		})

	return all, err
}

// EndpointRelation returns a Environment(Endpoint) relation object by EndpointID
func (service *Service) EndpointRelation(endpointID portainer.EndpointID) (*portainer.EndpointRelation, error) {
	var endpointRelation portainer.EndpointRelation
	identifier := service.connection.ConvertToKey(int(endpointID))

	err := service.connection.GetObject(BucketName, identifier, &endpointRelation)
	if err != nil {
		return nil, err
	}

	return &endpointRelation, nil
}

// CreateEndpointRelation saves endpointRelation
func (service *Service) Create(endpointRelation *portainer.EndpointRelation) error {
	err := service.connection.CreateObjectWithId(BucketName, int(endpointRelation.EndpointID), endpointRelation)
	cache.Del(endpointRelation.EndpointID)

	return err
}

// UpdateEndpointRelation updates an Environment(Endpoint) relation object
func (service *Service) UpdateEndpointRelation(endpointID portainer.EndpointID, endpointRelation *portainer.EndpointRelation) error {
	previousRelationState, _ := service.EndpointRelation(endpointID)

	identifier := service.connection.ConvertToKey(int(endpointID))
	err := service.connection.UpdateObject(BucketName, identifier, endpointRelation)
	cache.Del(endpointID)
	if err != nil {
		return err
	}

	updatedRelationState, _ := service.EndpointRelation(endpointID)

	service.updateEdgeStacksAfterRelationChange(previousRelationState, updatedRelationState)

	return nil
}

// DeleteEndpointRelation deletes an Environment(Endpoint) relation object
func (service *Service) DeleteEndpointRelation(endpointID portainer.EndpointID) error {
	deletedRelation, _ := service.EndpointRelation(endpointID)

	identifier := service.connection.ConvertToKey(int(endpointID))
	err := service.connection.DeleteObject(BucketName, identifier)
	cache.Del(endpointID)
	if err != nil {
		return err
	}

	service.updateEdgeStacksAfterRelationChange(deletedRelation, nil)

	return nil
}

func (service *Service) InvalidateEdgeCacheForEdgeStack(edgeStackID portainer.EdgeStackID) {
	rels, err := service.EndpointRelations()
	if err != nil {
		log.Error().Err(err).Msg("cannot retrieve endpoint relations")
		return
	}

	for _, rel := range rels {
		for id := range rel.EdgeStacks {
			if edgeStackID == id {
				cache.Del(rel.EndpointID)
			}
		}
	}
}

func (service *Service) updateEdgeStacksAfterRelationChange(previousRelationState *portainer.EndpointRelation, updatedRelationState *portainer.EndpointRelation) {
	relations, _ := service.EndpointRelations()

	stacksToUpdate := map[portainer.EdgeStackID]bool{}

	if previousRelationState != nil {
		for stackId, enabled := range previousRelationState.EdgeStacks {
			// flag stack for update if stack is not in the updated relation state
			// = stack has been removed for this relation
			// or this relation has been deleted
			if enabled && (updatedRelationState == nil || !updatedRelationState.EdgeStacks[stackId]) {
				stacksToUpdate[stackId] = true
			}
		}
	}

	if updatedRelationState != nil {
		for stackId, enabled := range updatedRelationState.EdgeStacks {
			// flag stack for update if stack is not in the previous relation state
			// = stack has been added for this relation
			if enabled && (previousRelationState == nil || !previousRelationState.EdgeStacks[stackId]) {
				stacksToUpdate[stackId] = true
			}
		}
	}

	// for each stack referenced by the updated relation
	// list how many time this stack is referenced in all relations
	// in order to update the stack deployments count
	for refStackId, refStackEnabled := range stacksToUpdate {
		if refStackEnabled {
			numDeployments := 0
			for _, r := range relations {
				for sId, enabled := range r.EdgeStacks {
					if enabled && sId == refStackId {
						numDeployments += 1
					}
				}
			}

			service.updateStackFn(refStackId, func(edgeStack *portainer.EdgeStack) {
				edgeStack.NumDeployments = numDeployments
			})
		}
	}
}
