package service

type ServiceStatus struct {
	BuildId string `json:"build_id"`
}

func (api *ControlAPI) hStatus(h *ControlAPIHandler) {
	status := ServiceStatus{
		BuildId: api.Service.Cfg.BuildId,
	}

	h.ReplyJSON(200, &status)
}
