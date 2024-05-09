package whatsmeow

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"

	whatsapp "github.com/nocodeleaks/quepasa/whatsapp"
	whatsmeow "go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	types "go.mau.fi/whatsmeow/types"
)

// Must Implement IWhatsappConnection
type WhatsmeowConnection struct {
	Client   *whatsmeow.Client
	Handlers *WhatsmeowHandlers

	failedToken bool
	paired      func(string)

	LogEntry *log.Entry `json:"-"` // log entry
}

//#region IMPLEMENT WHATSAPP CONNECTION OPTIONS INTERFACE

func (conn *WhatsmeowConnection) GetWid() string {
	if conn != nil {
		wid, err := conn.GetWidInternal()
		if err != nil {
			return wid
		}
	}

	return ""
}

// get default log entry, never nil
func (source *WhatsmeowConnection) GetLogger() *log.Entry {
	if source == nil {
		return log.WithContext(context.Background())
	}

	if source.LogEntry == nil {
		logger := log.StandardLogger()
		logger.SetLevel(log.DebugLevel)

		logentry := logger.WithContext(context.Background())

		wid, err := source.GetWidInternal()
		if err == nil && len(wid) > 0 {
			logentry = logentry.WithField("wid", wid)
		}

		source.LogEntry = logentry
	}

	return source.LogEntry
}

func (conn *WhatsmeowConnection) SetReconnect(value bool) {
	if conn != nil {
		if conn.Client != nil {
			conn.Client.EnableAutoReconnect = value
		}
	}
}

func (conn *WhatsmeowConnection) GetReconnect() bool {
	if conn != nil {
		if conn.Client != nil {
			return conn.Client.EnableAutoReconnect
		}
	}

	return false
}

//#endregion

//region IMPLEMENT INTERFACE WHATSAPP CONNECTION

func (conn *WhatsmeowConnection) GetVersion() string { return "multi" }

func (conn *WhatsmeowConnection) GetWidInternal() (wid string, err error) {
	if conn.Client == nil {
		err = fmt.Errorf("client not defined on trying to get wid")
	} else {
		if conn.Client.Store == nil {
			err = fmt.Errorf("device store not defined on trying to get wid")
		} else {
			if conn.Client.Store.ID == nil {
				err = fmt.Errorf("device id not defined on trying to get wid")
			} else {
				wid = conn.Client.Store.ID.User
			}
		}
	}

	return
}

func (conn *WhatsmeowConnection) IsValid() bool {
	if conn != nil {
		if conn.Client != nil {
			if conn.Client.IsConnected() {
				if conn.Client.IsLoggedIn() {
					return true
				}
			}
		}
	}
	return false
}

func (conn *WhatsmeowConnection) IsConnected() bool {
	if conn != nil {
		if conn.Client != nil {
			if conn.Client.IsConnected() {
				return true
			}
		}
	}
	return false
}

func (conn *WhatsmeowConnection) GetStatus() whatsapp.WhatsappConnectionState {
	if conn != nil {
		if conn.Client == nil {
			return whatsapp.UnVerified
		} else {
			if conn.Client.IsConnected() {
				if conn.Client.IsLoggedIn() {
					return whatsapp.Ready
				} else {
					return whatsapp.Connected
				}
			} else {
				if conn.failedToken {
					return whatsapp.Failed
				} else {
					return whatsapp.Disconnected
				}
			}
		}
	} else {
		return whatsapp.UnPrepared
	}
}

// returns a valid chat title from local memory store
func (conn *WhatsmeowConnection) GetChatTitle(wid string) string {
	jid, err := types.ParseJID(wid)
	if err == nil {
		return GetChatTitle(conn.Client, jid)
	}

	return ""
}

// Connect to websocket only, dot not authenticate yet, errors come after
func (source *WhatsmeowConnection) Connect() (err error) {
	source.GetLogger().Info("starting whatsmeow connection")

	err = source.Client.Connect()
	if err != nil {
		source.failedToken = true
		return
	}

	// waits 2 seconds for loggedin
	// not required
	_ = source.Client.WaitForConnection(time.Millisecond * 2000)

	source.failedToken = false
	return
}

// func (cli *Client) Download(msg DownloadableMessage) (data []byte, err error)
func (source *WhatsmeowConnection) DownloadData(imsg whatsapp.IWhatsappMessage) (data []byte, err error) {
	msg := imsg.GetSource()
	downloadable, ok := msg.(whatsmeow.DownloadableMessage)
	if !ok {
		source.GetLogger().Debug("not downloadable type, trying default message")
		waMsg, ok := msg.(*waProto.Message)
		if !ok {
			attach := imsg.GetAttachment()
			if attach != nil {
				data := attach.GetContent()
				if data != nil {
					return *data, err
				}
			}

			err = fmt.Errorf("parameter msg cannot be converted to an original message")
			return
		}
		return source.Client.DownloadAny(waMsg)
	}
	return source.Client.Download(downloadable)
}

func (conn *WhatsmeowConnection) Download(imsg whatsapp.IWhatsappMessage, cache bool) (att *whatsapp.WhatsappAttachment, err error) {
	att = imsg.GetAttachment()
	if att == nil {
		err = fmt.Errorf("message (%s) does not contains attachment info", imsg.GetId())
		return
	}

	if !att.HasContent() && !att.CanDownload {
		err = fmt.Errorf("message (%s) attachment with invalid content and not available to download", imsg.GetId())
		return
	}

	if !att.HasContent() || (att.CanDownload && !cache) {
		data, err := conn.DownloadData(imsg)
		if err != nil {
			return att, err
		}

		if !cache {
			newAtt := *att
			att = &newAtt
		}

		att.SetContent(&data)
	}

	return
}

func (source *WhatsmeowConnection) Revoke(msg whatsapp.IWhatsappMessage) error {
	jid, err := types.ParseJID(msg.GetChatId())
	if err != nil {
		source.GetLogger().Infof("revoke error on get jid: %s", err)
		return err
	}

	participantJid, err := types.ParseJID(msg.GetParticipantId())
	if err != nil {
		source.GetLogger().Infof("revoke error on get jid: %s", err)
		return err
	}

	newMessage := source.Client.BuildRevoke(jid, participantJid, msg.GetId())
	_, err = source.Client.SendMessage(context.Background(), jid, newMessage)
	if err != nil {
		source.GetLogger().Infof("revoke error: %s", err)
		return err
	}

	return nil
}

func (conn *WhatsmeowConnection) IsOnWhatsApp(phones ...string) (registered []string, err error) {
	results, err := conn.Client.IsOnWhatsApp(phones)
	if err != nil {
		return
	}

	for _, result := range results {
		if result.IsIn {
			registered = append(registered, result.JID.String())
		}
	}

	return
}

func (conn *WhatsmeowConnection) GetProfilePicture(wid string, knowingId string) (picture *whatsapp.WhatsappProfilePicture, err error) {
	jid, err := types.ParseJID(wid)
	if err != nil {
		return
	}

	params := &whatsmeow.GetProfilePictureParams{}
	params.ExistingID = knowingId
	params.Preview = false

	pictureInfo, err := conn.Client.GetProfilePictureInfo(jid, params)
	if err != nil {
		return
	}

	if pictureInfo != nil {
		picture = &whatsapp.WhatsappProfilePicture{
			Id:   pictureInfo.ID,
			Type: pictureInfo.Type,
			Url:  pictureInfo.URL,
		}
	}
	return
}

func isASCII(s string) bool {
	for _, c := range s {
		if c > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// Default SEND method using WhatsappMessage Interface
func (source *WhatsmeowConnection) Send(msg *whatsapp.WhatsappMessage) (whatsapp.IWhatsappSendResponse, error) {
	logentry := source.GetLogger()

	var err error

	// Formatting destination accordingly
	formattedDestination, _ := whatsapp.FormatEndpoint(msg.GetChatId())

	// avoid common issue with incorrect non ascii chat id
	if !isASCII(formattedDestination) {
		err = fmt.Errorf("not an ASCII formatted chat id")
		return msg, err
	}

	// validating jid before remote commands as upload or send
	jid, err := types.ParseJID(formattedDestination)
	if err != nil {
		logentry.Infof("send error on get jid: %s", err)
		return msg, err
	}

	// request message text
	messageText := msg.GetText()

	var newMessage *waProto.Message
	if !msg.HasAttachment() {
		if IsValidForButtons(messageText) {
			internal := GenerateButtonsMessage(messageText)
			newMessage = &waProto.Message{ButtonsMessage: internal}
		} else {
			internal := &waProto.ExtendedTextMessage{Text: &messageText}
			if len(msg.InReply) > 0 {

				var sender *string
				if msg.FromGroup() {
					// getting connection store id for use as group participant
					storeid := source.Client.Store.ID

					// formating sender without device and session info
					jid := fmt.Sprintf("%s@%s", storeid.User, storeid.Server)
					sender = proto.String(jid)
				}

				// getting quoted message if available on cache
				// (optional) another devices will process anyway, but our devices will show quoted only if it exists on cache
				var quoted *waProto.Message
				cached, _ := source.Handlers.WAHandlers.GetMessage(msg.InReply)
				if cached.Content != nil {
					if internal, ok := cached.Content.(*waProto.Message); ok {
						quoted = internal
					} else {
						logentry.Warnf("content has an invalid type (%s), on reply to msg id: %s", reflect.TypeOf(cached.Content), msg.InReply)
					}
				} else {
					logentry.Warnf("message not cached, on reply to msg id: %s", msg.InReply)
				}

				internal.ContextInfo = &waProto.ContextInfo{
					StanzaId:      &msg.InReply,
					Participant:   sender,
					QuotedMessage: quoted,
				}
			}
			newMessage = &waProto.Message{ExtendedTextMessage: internal}
		}
	} else {
		newMessage, err = source.UploadAttachment(*msg)
		if err != nil {
			return msg, err
		}
	}

	// Generating a new unique MessageID
	if len(msg.Id) == 0 {
		msg.Id = source.Client.GenerateMessageID()
	}

	extra := whatsmeow.SendRequestExtra{
		ID: msg.Id,
	}

	resp, err := source.Client.SendMessage(context.Background(), jid, newMessage, extra)
	if err != nil {
		logentry.Errorf("send error: %s", err)
		return msg, err
	}

	// updating timestamp
	msg.Timestamp = resp.Timestamp

	if msg.Id != resp.ID {
		logentry.Warnf("send success but msg id: %s, differs from response id: %s, type: %v, on: %s", msg.Id, resp.ID, msg.Type, msg.Timestamp)
	} else {
		logentry.Infof("send success, msg id: %s, type: %v, on: %s", msg.Id, msg.Type, msg.Timestamp)
	}

	return msg, err
}

// func (cli *Client) Upload(ctx context.Context, plaintext []byte, appInfo MediaType) (resp UploadResponse, err error)
func (conn *WhatsmeowConnection) UploadAttachment(msg whatsapp.WhatsappMessage) (result *waProto.Message, err error) {

	content := *msg.Attachment.GetContent()
	if len(content) == 0 {
		err = fmt.Errorf("null or empty content")
		return
	}

	mediaType := GetMediaTypeFromWAMsgType(msg.Type)
	response, err := conn.Client.Upload(context.Background(), content, mediaType)
	if err != nil {
		return
	}

	result = NewWhatsmeowMessageAttachment(response, msg, mediaType)
	return
}

func (conn *WhatsmeowConnection) Disconnect() (err error) {
	if conn.Client != nil {
		if conn.Client.IsConnected() {
			conn.Client.Disconnect()
		}
	}
	return
}

func (source *WhatsmeowConnection) GetInvite(groupId string) (link string, err error) {
	jid, err := types.ParseJID(groupId)
	if err != nil {
		source.GetLogger().Infof("getting invite error on parse jid: %s", err)
	}

	link, err = source.Client.GetGroupInviteLink(jid, false)
	return
}

func (conn *WhatsmeowConnection) GetWhatsAppQRCode() string {

	var result string

	// No ID stored, new login
	qrChan, err := conn.Client.GetQRChannel(context.Background())
	if err != nil {
		log.Errorf("error on getting whatsapp qrcode channel: %s", err.Error())
		return ""
	}

	if !conn.Client.IsConnected() {
		err = conn.Client.Connect()
		if err != nil {
			log.Errorf("error on connecting for getting whatsapp qrcode: %s", err.Error())
			return ""
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)

	for evt := range qrChan {
		if evt.Event == "code" {
			result = evt.Code
		}

		wg.Done()
		break
	}

	wg.Wait()
	return result
}

func TryUpdateChannel(ch chan<- string, value string) (closed bool) {
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

func (source *WhatsmeowConnection) GetWhatsAppQRChannel(ctx context.Context, out chan<- string) error {
	logger := source.GetLogger()

	// No ID stored, new login
	qrChan, err := source.Client.GetQRChannel(ctx)
	if err != nil {
		logger.Errorf("error on getting whatsapp qrcode channel: %s", err.Error())
		return err
	}

	if !source.Client.IsConnected() {
		err = source.Client.Connect()
		if err != nil {
			logger.Errorf("error on connecting for getting whatsapp qrcode: %s", err.Error())
			return err
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)

	for evt := range qrChan {
		if evt.Event == "code" {
			if !TryUpdateChannel(out, evt.Code) {
				// expected error, means that websocket was closed
				// probably user has gone out page
				return fmt.Errorf("cant write to output")
			}
		} else {
			if evt.Event == "timeout" {
				return errors.New("timeout")
			}
			wg.Done()
			break
		}
	}

	wg.Wait()
	return nil
}

func (source *WhatsmeowConnection) HistorySync(timestamp time.Time) (err error) {
	logentry := source.GetLogger()

	leading := source.Handlers.WAHandlers.GetLeadingMessage()
	if leading == nil {
		err = fmt.Errorf("no valid msg in cache for retrieve parents")
		return err
	}

	// Convert interface to struct using type assertion
	info, ok := leading.Info.(types.MessageInfo)
	if !ok {
		logentry.Error("error converting leading for history")
	}

	logentry.Infof("getting history from: %s", timestamp)
	extra := whatsmeow.SendRequestExtra{Peer: true}

	//info := &types.MessageInfo{ }
	msg := source.Client.BuildHistorySyncRequest(&info, 50)
	response, err := source.Client.SendMessage(context.Background(), source.Client.Store.ID.ToNonAD(), msg, extra)
	if err != nil {
		logentry.Errorf("getting history error: %s", err.Error())
	}

	logentry.Infof("history: %v", response)
	return
}

func (conn *WhatsmeowConnection) UpdateHandler(handlers whatsapp.IWhatsappHandlers) {
	conn.Handlers.WAHandlers = handlers
}

func (conn *WhatsmeowConnection) UpdatePairedCallBack(callback func(string)) {
	conn.paired = callback
}

func (conn *WhatsmeowConnection) PairedCallBack(jid types.JID, platform, businessName string) bool {
	if conn.paired != nil {
		go conn.paired(jid.String())
	}
	return true
}

//endregion

/*
<summary>

	Disconnect if connected
	Cleanup Handlers
	Dispose resources
	Does not erase permanent data !

</summary>
*/
func (source *WhatsmeowConnection) Dispose(reason string) {

	source.GetLogger().Infof("disposing connection: %s", reason)

	if source.Handlers != nil {
		go source.Handlers.UnRegister()
		source.Handlers = nil
	}

	if source.Client != nil {
		if source.Client.IsConnected() {
			go source.Client.Disconnect()
		}
		source.Client = nil
	}

	source = nil
}

/*
<summary>

	Erase permanent data + Dispose !

</summary>
*/
func (source *WhatsmeowConnection) Delete() (err error) {
	if source != nil {
		if source.Client != nil {
			if source.Client.IsLoggedIn() {
				err = source.Client.Logout()
				if err != nil {
					return
				}
				source.GetLogger().Infof("logged out for delete")
			}

			if source.Client.Store != nil {
				err = source.Client.Store.Delete()
				if err != nil {
					// ignoring error about JID, just checked and the delete process was succeed
					if strings.Contains(err.Error(), "device JID must be known before accessing database") {
						err = nil
					} else {
						err = fmt.Errorf("error on trying to delete store: %s", err.Error())
						return
					}
				}
			}
		}
	}

	source.Dispose("Delete")
	return
}

func (conn *WhatsmeowConnection) IsInterfaceNil() bool {
	return nil == conn
}
