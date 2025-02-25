package edgejobs

import (
	"net/http"
	"strconv"

	httperror "github.com/portainer/libhttp/error"
	"github.com/portainer/libhttp/request"
	"github.com/portainer/libhttp/response"
	portainer "github.com/portainer/portainer/api"
	"github.com/portainer/portainer/api/internal/edge"
	"github.com/portainer/portainer/api/internal/slices"
)

// @id EdgeJobTasksClear
// @summary Clear the log for a specifc task on an EdgeJob
// @description **Access policy**: administrator
// @tags edge_jobs
// @security ApiKeyAuth
// @security jwt
// @produce json
// @param id path string true "EdgeJob Id"
// @param taskID path string true "Task Id"
// @success 204
// @failure 500
// @failure 400
// @failure 503 "Edge compute features are disabled"
// @router /edge_jobs/{id}/tasks/{taskID}/logs [delete]
func (handler *Handler) edgeJobTasksClear(w http.ResponseWriter, r *http.Request) *httperror.HandlerError {
	edgeJobID, err := request.RetrieveNumericRouteVariableValue(r, "id")
	if err != nil {
		return httperror.BadRequest("Invalid Edge job identifier route variable", err)
	}

	taskID, err := request.RetrieveNumericRouteVariableValue(r, "taskID")
	if err != nil {
		return httperror.BadRequest("Invalid Task identifier route variable", err)
	}

	edgeJob, err := handler.DataStore.EdgeJob().EdgeJob(portainer.EdgeJobID(edgeJobID))
	if handler.DataStore.IsErrObjectNotFound(err) {
		return httperror.NotFound("Unable to find an Edge job with the specified identifier inside the database", err)
	} else if err != nil {
		return httperror.InternalServerError("Unable to find an Edge job with the specified identifier inside the database", err)
	}

	endpointID := portainer.EndpointID(taskID)
	endpointsFromGroups, err := edge.GetEndpointsFromEdgeGroups(edgeJob.EdgeGroups, handler.DataStore)
	if err != nil {
		return httperror.InternalServerError("Unable to get Endpoints from EdgeGroups", err)
	}

	if slices.Contains(endpointsFromGroups, endpointID) {
		edgeJob.GroupLogsCollection[endpointID] = portainer.EdgeJobEndpointMeta{
			CollectLogs: false,
			LogsStatus:  portainer.EdgeJobLogsStatusIdle,
		}
	} else {
		meta := edgeJob.Endpoints[endpointID]
		meta.CollectLogs = false
		meta.LogsStatus = portainer.EdgeJobLogsStatusIdle
		edgeJob.Endpoints[endpointID] = meta
	}

	err = handler.FileService.ClearEdgeJobTaskLogs(strconv.Itoa(edgeJobID), strconv.Itoa(taskID))
	if err != nil {
		return httperror.InternalServerError("Unable to clear log file from disk", err)
	}

	endpoint, err := handler.DataStore.Endpoint().Endpoint(endpointID)
	if err != nil {
		return httperror.NotFound("Unable to retrieve environment from the database", err)
	}

	handler.ReverseTunnelService.AddEdgeJob(endpoint, edgeJob)

	err = handler.DataStore.EdgeJob().UpdateEdgeJob(edgeJob.ID, edgeJob)
	if err != nil {
		return httperror.InternalServerError("Unable to persist Edge job changes in the database", err)
	}

	return response.Empty(w)
}
