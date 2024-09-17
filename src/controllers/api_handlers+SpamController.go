package controllers

import (
	"net/http"

	metrics "github.com/nocodeleaks/quepasa/metrics"
	models "github.com/nocodeleaks/quepasa/models"
)

// -------------------------- PUBLIC METHODS
//region TYPES OF SPAMMING

// SendAPIHandler renders route "/v4/bot/{token}/spam"
func Spam(w http.ResponseWriter, r *http.Request) {
	server, err := GetServerFromMaster(r)
	if err != nil {
		metrics.MessageSendErrors.Inc()

		response := &models.QpSendResponse{}
		response.ParseError(err)
		RespondInterface(w, response)
		return
	}

	SendAnyWithServer(w, r, server)
}