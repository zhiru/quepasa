package controllers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"

	models "github.com/nocodeleaks/quepasa/models"
)

const CurrentAPIVersion string = "v4"

func RegisterAPIControllers(r chi.Router) {
	aliases := []string{"/current", "", "/" + CurrentAPIVersion}
	for _, endpoint := range aliases {

		// CONTROL METHODS ************************
		// ----------------------------------------
		r.Get(endpoint+"/info", InformationController)
		r.Patch(endpoint+"/info", InformationController)
		r.Delete(endpoint+"/info", InformationController)

		r.Get(endpoint+"/scan", ScannerController)
		r.Get(endpoint+"/command", CommandController)

		// ----------------------------------------
		// CONTROL METHODS ************************

		// SENDING MSG ----------------------------
		// ----------------------------------------

		r.Get(endpoint+"/message/{messageid}", GetMessageController)
		r.Get(endpoint+"/message", GetMessageController)

		r.Delete(endpoint+"/message/{messageid}", RevokeController)
		r.Delete(endpoint+"/message", RevokeController)

		// used to dispatch alert msgs via url, triggers on monitor systems like zabbix
		r.Get(endpoint+"/send", SendAny)

		r.Post(endpoint+"/send", SendAny)
		r.Post(endpoint+"/send/{chatid}", SendAny)
		r.Post(endpoint+"/sendtext", SendText)
		r.Post(endpoint+"/sendtext/{chatid}", SendText)

		// SENDING MSG ATTACH ---------------------

		// deprecated, discard/remove on next version
		r.Post(endpoint+"/senddocument", SendDocumentAPIHandlerV2)

		r.Post(endpoint+"/sendurl", SendAnyFromUrl)
		r.Post(endpoint+"/sendbinary/{chatid}/{filename}/{text}", SendDocumentFromBinary)
		r.Post(endpoint+"/sendbinary/{chatid}/{filename}", SendDocumentFromBinary)
		r.Post(endpoint+"/sendbinary/{chatid}", SendDocumentFromBinary)
		r.Post(endpoint+"/sendbinary", SendDocumentFromBinary)
		r.Post(endpoint+"/sendencoded", SendDocumentFromEncoded)

		// ----------------------------------------
		// SENDING MSG ----------------------------

		r.Get(endpoint+"/receive", ReceiveAPIHandler)
		r.Post(endpoint+"/attachment", AttachmentAPIHandlerV2)

		r.Get(endpoint+"/download/{messageid}", DownloadController)
		r.Get(endpoint+"/download", DownloadController)

		// PICTURE INFO | DATA --------------------
		// ----------------------------------------

		r.Post(endpoint+"/picinfo", PictureController)
		r.Get(endpoint+"/picinfo/{chatid}/{pictureid}", PictureController)
		r.Get(endpoint+"/picinfo/{chatid}", PictureController)
		r.Get(endpoint+"/picinfo", PictureController)

		r.Post(endpoint+"/picdata", PictureController)
		r.Get(endpoint+"/picdata/{chatid}/{pictureid}", PictureController)
		r.Get(endpoint+"/picdata/{chatid}", PictureController)
		r.Get(endpoint+"/picdata", PictureController)

		// ----------------------------------------
		// PICTURE INFO | DATA --------------------

		r.Post(endpoint+"/webhook", WebhookController)
		r.Get(endpoint+"/webhook", WebhookController)
		r.Delete(endpoint+"/webhook", WebhookController)

		// INVITE METHODS ************************
		// ----------------------------------------

		r.Get(endpoint+"/invite", InviteController)
		r.Get(endpoint+"/invite/{chatid}", InviteController)

		// ----------------------------------------
		// INVITE METHODS ************************
	}
}

func ScannerController(w http.ResponseWriter, r *http.Request) {
	// setting default response type as json
	w.Header().Set("Content-Type", "application/json")

	response := &models.QpResponse{}

	token := GetToken(r)
	if len(token) == 0 {
		err := fmt.Errorf("token not found")
		RespondBadRequest(w, err)
		return
	}

	user, err := GetUser(r)
	if err != nil {
		err := fmt.Errorf("user not found: %s", err.Error())
		response.ParseError(err)
		RespondInterface(w, response)
		return
	}

	pairing := &models.QpWhatsappPairing{Token: token, User: user}
	con, err := pairing.GetConnection()
	if err != nil {
		err := fmt.Errorf("cant get connection: %s", err.Error())
		response.ParseError(err)
		RespondInterface(w, response)
		return
	}

	log.Infof("requesting qrcode for token %s", token)
	result := con.GetWhatsAppQRCode()

	var png []byte
	png, err = qrcode.Encode(result, qrcode.Medium, 256)
	if err != nil {
		err := fmt.Errorf("cant get qrcode: %s", err.Error())
		response.ParseError(err)
		RespondInterface(w, response)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=qrcode.png")
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(png))
}

func CommandController(w http.ResponseWriter, r *http.Request) {
	// setting default response type as json
	w.Header().Set("Content-Type", "application/json")

	response := &models.QpResponse{}

	server, err := GetServer(r)
	if err != nil {
		response.ParseError(err)
		RespondInterface(w, response)
		return
	}

	action := models.GetRequestParameter(r, "action")
	switch action {
	case "start":
		err = server.Start()
		if err == nil {
			response.ParseSuccess("started")
		}
	case "stop":
		err = server.Stop("command")
		if err == nil {
			response.ParseSuccess("stopped")
		}
	case "restart":
		err = server.Restart()
		if err == nil {
			response.ParseSuccess("restarted")
		}
	case "status":
		status := server.GetStatus()
		response.ParseSuccess(status.String())
	case "groups":
		err := models.ToggleGroups(server)
		if err == nil {
			message := "groups toggled: " + server.Groups.String()
			response.ParseSuccess(message)
		}
	case "broadcasts":
		err := models.ToggleBroadcasts(server)
		if err == nil {
			message := "broadcasts toggled: " + server.Broadcasts.String()
			response.ParseSuccess(message)
		}
	case "readreceipts":
		err := models.ToggleReadReceipts(server)
		if err == nil {
			message := "readreceipts toggled: " + server.ReadReceipts.String()
			response.ParseSuccess(message)
		}
	case "calls":
		err := models.ToggleCalls(server)
		if err == nil {
			message := "calls toggled: " + server.Calls.String()
			response.ParseSuccess(message)
		}
	default:
		err = fmt.Errorf("invalid action: {%s}, try {start,stop,restart,status,groups}", action)
	}

	if err != nil {
		response.ParseError(err)
	}

	RespondInterface(w, response)
}
