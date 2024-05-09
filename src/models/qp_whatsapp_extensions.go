package models

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"

	whatsapp "github.com/nocodeleaks/quepasa/whatsapp"
)

func NewEmptyConnection(callback func(string)) (conn whatsapp.IWhatsappConnection, err error) {
	return NewWhatsmeowEmptyConnection(callback)
}

func NewConnection(options *whatsapp.WhatsappConnectionOptions) (whatsapp.IWhatsappConnection, error) {
	return NewWhatsmeowConnection(options)
}

func TryUpdateHttpChannel(ch chan<- []byte, value []byte) (closed bool) {
	defer func() {
		if recover() != nil {
			// the return result can be altered
			// in a defer function call
			closed = false
		}
	}()

	ch <- value // panic if ch is closed
	return true // <=> closed = false; return
}

// Envia o QRCode para o usuário e aguarda pela resposta
// Retorna um novo BOT
func SignInWithQRCode(ctx context.Context, user *QpUser, out chan<- []byte) (err error) {

	pairing := &QpWhatsappPairing{User: user}
	con, err := pairing.GetConnection()
	if err != nil {
		return
	}

	logger := con.GetLogger()
	qrChan := make(chan string)
	defer close(qrChan)
	go func() {
		for qrBase64 := range qrChan {
			var png []byte
			png, err := qrcode.Encode(qrBase64, qrcode.Medium, 256)
			if err != nil {
				logger.Errorf("(qrcode) encode fail, %s", err.Error())
				return
			}

			encodedPNG := base64.StdEncoding.EncodeToString(png)
			if !TryUpdateHttpChannel(out, []byte(encodedPNG)) {
				// expected error, means that websocket was closed
				// probably user has gone out page
				logger.Error("(qrcode) cant write to output")
				return
			}
		}
	}()

	logger.Info("(qrcode) getting qrcode channel ...")
	return con.GetWhatsAppQRChannel(ctx, qrChan)
}

func EnsureServerOnCache(currentUserID string, wid string, connection whatsapp.IWhatsappConnection) (err error) {
	// Se chegou até aqui é pq o QRCode foi validado e sincronizado
	server, err := WhatsappService.GetOrCreateServer(currentUserID, wid)
	if err != nil {
		log.Errorf("getting or create server after login : %s", err.Error())
		return
	}

	// updating verified state
	server.MarkVerified(true)

	// updating underlying connection
	go server.UpdateConnection(connection)
	return
}

func GetDownloadPrefixFromToken(token string) (path string, err error) {
	server, ok := WhatsappService.Servers[token]
	if !ok {
		err = fmt.Errorf("server not found: %s", token)
		return
	}

	prefix := fmt.Sprintf("/bot/%s/download", server.Token)
	return prefix, err
}

func ToQPAttachmentV1(source *whatsapp.WhatsappAttachment, id string, token string) (attach *QPAttachmentV1) {

	// Anexo que devolverá ao utilizador da api, cliente final
	// com Url pública válida sem criptografia
	attach = &QPAttachmentV1{}
	attach.MIME = source.Mimetype
	attach.FileName = source.FileName
	attach.Length = source.FileLength

	url, err := GetDownloadPrefixFromToken(token)
	if err != nil {
		return
	}

	attach.Url = url + "/" + id
	return
}

func ToQPEndPointV1(source *whatsapp.WhatsappEndpoint) (destination QPEndpointV1) {
	if !strings.Contains(source.ID, "@") {
		if source.ID == "status" {
			destination.ID = source.ID + "@broadcast"
		} else if strings.Contains(source.ID, "-") {
			destination.ID = source.ID + "@g.us"
		} else {
			destination.ID = source.ID + "@s.whatsapp.net"
		}
	} else {
		destination.ID = source.ID
	}

	destination.Title = source.Title
	if len(destination.Title) == 0 {
		destination.Title = source.UserName
	}

	return
}

func ToQPEndPointV2(source *whatsapp.WhatsappEndpoint) (destination QPEndpointV2) {
	if !strings.Contains(source.ID, "@") {
		if source.ID == "status" {
			destination.ID = source.ID + "@broadcast"
		} else if strings.Contains(source.ID, "-") {
			destination.ID = source.ID + "@g.us"
		} else {
			destination.ID = source.ID + "@s.whatsapp.net"
		}
	} else {
		destination.ID = source.ID
	}

	destination.Title = source.Title
	if len(destination.Title) == 0 {
		destination.Title = source.UserName
	}

	return
}

func ChatToQPEndPointV1(source whatsapp.WhatsappChat) (destination QPEndpointV1) {
	if !strings.Contains(source.Id, "@") {
		if source.Id == "status" {
			destination.ID = source.Id + "@broadcast"
		} else if strings.Contains(source.Id, "-") {
			destination.ID = source.Id + "@g.us"
		} else {
			destination.ID = source.Id + "@s.whatsapp.net"
		}
	} else {
		destination.ID = source.Id
	}

	destination.Title = source.Title
	return
}

func ChatToQPChatV2(source whatsapp.WhatsappChat) (destination QPChatV2) {
	if !strings.Contains(source.Id, "@") {
		if source.Id == "status" {
			destination.ID = source.Id + "@broadcast"
		} else if strings.Contains(source.Id, "-") {
			destination.ID = source.Id + "@g.us"
		} else {
			destination.ID = source.Id + "@s.whatsapp.net"
		}
	} else {
		destination.ID = source.Id
	}

	destination.Title = source.Title
	return
}

func ChatToQPEndPointV2(source whatsapp.WhatsappChat) (destination QPEndpointV2) {
	if !strings.Contains(source.Id, "@") {
		if source.Id == "status" {
			destination.ID = source.Id + "@broadcast"
		} else if strings.Contains(source.Id, "-") {
			destination.ID = source.Id + "@g.us"
		} else {
			destination.ID = source.Id + "@s.whatsapp.net"
			destination.UserName = "+" + source.Id
		}
	} else {
		destination.ID = source.Id
	}

	destination.Title = source.Title
	return
}

func ToWhatsappMessage(destination string, text string, attach *whatsapp.WhatsappAttachment) (msg *whatsapp.WhatsappMessage, err error) {
	recipient, err := whatsapp.FormatEndpoint(destination)
	if err != nil {
		return
	}

	msg = &whatsapp.WhatsappMessage{}
	msg.FromInternal = true
	msg.FromMe = true
	msg.Type = whatsapp.TextMessageType
	msg.Text = text

	chat := whatsapp.WhatsappChat{Id: recipient}
	msg.Chat = chat

	if attach != nil {
		msg.Attachment = attach
		msg.Type = whatsapp.GetMessageType(attach)
	}
	return

}

//#region USING WHATSAPP OPTIONS INTERFACE

func ToggleReadReceipts(source whatsapp.IWhatsappOptions) error {
	options := source.GetOptions()

	if options.ReadReceipts == whatsapp.UnSetBooleanType {
		options.ReadReceipts = whatsapp.TrueBooleanType
	} else if options.ReadReceipts == whatsapp.TrueBooleanType {
		options.ReadReceipts = whatsapp.FalseBooleanType
	} else {
		options.ReadReceipts = whatsapp.UnSetBooleanType
	}
	return source.Save()
}

func ToggleGroups(source whatsapp.IWhatsappOptions) error {
	options := source.GetOptions()

	if options.Groups == whatsapp.UnSetBooleanType {
		options.Groups = whatsapp.TrueBooleanType
	} else if options.Groups == whatsapp.TrueBooleanType {
		options.Groups = whatsapp.FalseBooleanType
	} else {
		options.Groups = whatsapp.UnSetBooleanType
	}
	return source.Save()
}

func ToggleBroadcasts(source whatsapp.IWhatsappOptions) error {
	options := source.GetOptions()

	if options.Broadcasts == whatsapp.UnSetBooleanType {
		options.Broadcasts = whatsapp.TrueBooleanType
	} else if options.Broadcasts == whatsapp.TrueBooleanType {
		options.Broadcasts = whatsapp.FalseBooleanType
	} else {
		options.Broadcasts = whatsapp.UnSetBooleanType
	}
	return source.Save()
}

func ToggleCalls(source whatsapp.IWhatsappOptions) error {
	options := source.GetOptions()

	if options.Calls == whatsapp.UnSetBooleanType {
		options.Calls = whatsapp.TrueBooleanType
	} else if options.Calls == whatsapp.TrueBooleanType {
		options.Calls = whatsapp.FalseBooleanType
	} else {
		options.Calls = whatsapp.UnSetBooleanType
	}
	return source.Save()
}

//#endregion
