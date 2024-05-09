package whatsmeow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	library "github.com/nocodeleaks/quepasa/library"
	whatsapp "github.com/nocodeleaks/quepasa/whatsapp"
	log "github.com/sirupsen/logrus"
	whatsmeow "go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/binary"
	types "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type WhatsmeowHandlers struct {
	// particular whatsapp options for this handler
	*whatsapp.WhatsappOptions

	// default whatsmeow service global options
	WhatsmeowOptions

	Client     *whatsmeow.Client
	WAHandlers whatsapp.IWhatsappHandlers

	eventHandlerID           uint32
	unregisterRequestedToken bool
	service                  *WhatsmeowServiceModel

	LogEntry *log.Entry
}

// get default log entry, never nil
func (source *WhatsmeowHandlers) GetLogger() *log.Entry {
	if source.LogEntry != nil {
		return source.LogEntry
	}

	return log.WithContext(context.Background())
}

func (source *WhatsmeowHandlers) GetServiceOptions() (options whatsapp.WhatsappOptionsExtended) {
	if source != nil {
		return source.WhatsappOptionsExtended
	}

	return
}

//#region WHATSAPP OPTIONS

func (source *WhatsmeowHandlers) HandleBroadcasts() bool {
	if source == nil {
		return false
	}

	var defaultValue whatsapp.WhatsappBoolean
	if source.WhatsappOptions != nil {
		defaultValue = source.WhatsappOptions.Broadcasts
	}

	serviceOptions := source.GetServiceOptions()
	return serviceOptions.HandleBroadcasts(defaultValue)
}

func (source *WhatsmeowHandlers) HandleGroups() bool {
	if source == nil {
		return false
	}

	var defaultValue whatsapp.WhatsappBoolean
	if source.WhatsappOptions != nil {
		defaultValue = source.WhatsappOptions.Groups
	}

	serviceOptions := source.GetServiceOptions()
	return serviceOptions.HandleGroups(defaultValue)
}

func (source *WhatsmeowHandlers) HandleReadReceipts() bool {
	if source == nil {
		return false
	}

	var defaultValue whatsapp.WhatsappBoolean
	if source.WhatsappOptions != nil {
		defaultValue = source.WhatsappOptions.ReadReceipts
	}

	serviceOptions := source.GetServiceOptions()
	return serviceOptions.HandleReadReceipts(defaultValue)
}

func (source *WhatsmeowHandlers) HandleCalls() bool {
	if source == nil {
		return false
	}

	var defaultValue whatsapp.WhatsappBoolean
	if source.WhatsappOptions != nil {
		defaultValue = source.WhatsappOptions.Calls
	}

	serviceOptions := source.GetServiceOptions()
	return serviceOptions.HandleCalls(defaultValue)
}

//#endregion

func (source WhatsmeowHandlers) HandleHistorySync() bool {
	options := source.GetServiceOptions()
	if options.HistorySync != nil {
		return true
	}

	return whatsapp.WhatsappHistorySync
}

// only affects whatsmeow
func (handler *WhatsmeowHandlers) UnRegister() {
	handler.unregisterRequestedToken = true

	// if is this session
	found := handler.Client.RemoveEventHandler(handler.eventHandlerID)
	if found {
		handler.GetLogger().Infof("handler unregistered, id: %v", handler.eventHandlerID)
	}
}

func (source *WhatsmeowHandlers) Register() (err error) {
	if source.Client.Store == nil {
		err = fmt.Errorf("this client lost the store, probably a logout from whatsapp phone")
		return
	}

	source.unregisterRequestedToken = false
	source.eventHandlerID = source.Client.AddEventHandler(source.EventsHandler)

	logger := source.GetLogger()
	logger.Infof("handler registered, id: %v", source.eventHandlerID)

	return
}

var historySyncID int32
var startupTime = time.Now().Unix()

// Define os diferentes tipos de eventos a serem reconhecidos
// Aqui se define se vamos processar mensagens | confirmações de leitura | etc
func (source *WhatsmeowHandlers) EventsHandler(rawEvt interface{}) {
	if source == nil {
		return
	}

	logger := source.GetLogger()

	if source.unregisterRequestedToken {
		logger.Info("unregister event handler requested")
		source.Client.RemoveEventHandler(source.eventHandlerID)
		return
	}

	switch evt := rawEvt.(type) {

	case *events.Message:
		go source.Message(*evt)
		return

	case *events.CallOffer:
		go source.CallMessage(evt.BasicCallMeta)
		return

	case *events.CallOfferNotice:
		go source.CallMessage(evt.BasicCallMeta)
		return

	case *events.Receipt:
		if source.HandleReadReceipts() {
			go source.Receipt(*evt)
		}
		return

	case *events.Connected:
		if source.Client != nil {
			// zerando contador de tentativas de reconexão
			// importante para zerar o tempo entre tentativas em caso de erro
			source.Client.AutoReconnectErrors = 0

			PushNameSetting(source.Client, logger)
		}

		if source.WAHandlers != nil && !source.WAHandlers.IsInterfaceNil() {
			go source.WAHandlers.OnConnected()
		}
		return

	case *events.PushNameSetting:
		PushNameSetting(source.Client, logger)
		return

	case *events.Disconnected:
		msgDisconnected := "disconnected from server"
		if source.Client.EnableAutoReconnect {
			logger.Info(msgDisconnected + ", dont worry, reconnecting")
		} else {
			logger.Warn(msgDisconnected)
		}

		if source.WAHandlers != nil && !source.WAHandlers.IsInterfaceNil() {
			go source.WAHandlers.OnDisconnected()
		}
		return

	case *events.LoggedOut:
		source.OnLoggedOutEvent(*evt)
		return

	case *events.HistorySync:
		if source.HandleHistorySync() {
			go source.OnHistorySyncEvent(*evt)
		}
		return

	case *events.AppStateSyncComplete:
		if len(source.Client.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
			err := source.Client.SendPresence(types.PresenceAvailable)
			if err != nil {
				logger.Warnf("failed to send available presence: %v", err)
			} else {
				logger.Debug("marked self as available from app state sync")
			}
		}

	case
		*events.AppState,
		*events.CallTerminate,
		*events.Contact,
		*events.DeleteChat,
		*events.DeleteForMe,
		*events.MarkChatAsRead,
		*events.Mute,
		*events.OfflineSyncCompleted,
		*events.OfflineSyncPreview,
		*events.PairSuccess,
		*events.Pin,
		*events.PushName,
		*events.GroupInfo,
		*events.QR:
		logger.Tracef("event ignored: %v", reflect.TypeOf(evt))
		return // ignoring not implemented yet

	default:
		logger.Debugf("event not handled: %v", reflect.TypeOf(evt))
		return
	}
}

func PushNameSetting(cli *whatsmeow.Client, logger *log.Entry) {
	if len(cli.Store.PushName) == 0 {
		return
	}
	// Send presence available when connecting and when the pushname is changed.
	// This makes sure that outgoing messages always have the right pushname.
	err := cli.SendPresence(types.PresenceAvailable)
	if err != nil {
		logger.Warnf("failed to send available presence: %v", err)
	} else {
		logger.Debug("marked self as available")
	}
}

func HistorySyncSaveJSON(evt events.HistorySync) {
	id := atomic.AddInt32(&historySyncID, 1)
	fileName := fmt.Sprintf("history-%d-%d.json", startupTime, id)
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		log.Errorf("Failed to open file to write history sync: %v", err)
		return
	}
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	err = enc.Encode(evt.Data)
	if err != nil {
		log.Errorf("Failed to write history sync: %v", err)
		return
	}
	log.Infof("Wrote history sync to %s", fileName)
	_ = file.Close()
}

func (source *WhatsmeowHandlers) OnHistorySyncEvent(evt events.HistorySync) {
	logentry := source.GetLogger()
	logentry.Infof("history sync: %s", evt.Data.SyncType)
	// HistorySyncSaveJSON(evt)

	conversations := evt.Data.GetConversations()
	for _, conversation := range conversations {
		for _, historyMsg := range conversation.GetMessages() {
			wid, err := types.ParseJID(conversation.GetId())
			if err != nil {
				logentry.Errorf("failed to parse jid at history sync: %v", err)
				return
			}

			msgevt, err := source.Client.ParseWebMessage(wid, historyMsg.GetMessage())
			if err != nil {
				logentry.Errorf("failed to parse web message at history sync: %v", err)
				return
			}

			source.Message(*msgevt)
		}
	}
}

//#region EVENT MESSAGE

// Aqui se processar um evento de recebimento de uma mensagem genérica
func (handler *WhatsmeowHandlers) Message(evt events.Message) {
	logentry := handler.GetLogger()
	logentry.Trace("event message received")

	if evt.Message == nil {
		if evt.SourceWebMsg != nil {
			// probably from recover history sync
			logentry.Info("web message cant be full decrypted, ignoring")
			return
		}

		jsonstring, _ := json.Marshal(evt)
		logentry.Errorf("nil message on receiving whatsmeow events | try use rawMessage ! json: %s", string(jsonstring))
		return
	}

	message := &whatsapp.WhatsappMessage{
		Content: evt.Message,
		Info:    evt.Info,
	}

	// basic information
	message.Id = evt.Info.ID
	message.Timestamp = evt.Info.Timestamp
	message.FromMe = evt.Info.IsFromMe

	message.Chat = whatsapp.WhatsappChat{}
	chatID := fmt.Sprint(evt.Info.Chat.User, "@", evt.Info.Chat.Server)
	message.Chat.Id = chatID
	message.Chat.Title = GetChatTitle(handler.Client, evt.Info.Chat)

	if evt.Info.IsGroup {
		message.Participant = &whatsapp.WhatsappChat{}

		participantID := fmt.Sprint(evt.Info.Sender.User, "@", evt.Info.Sender.Server)
		message.Participant.Id = participantID
		message.Participant.Title = GetChatTitle(handler.Client, evt.Info.Sender)

		// sugested by hugo sampaio, removing message.FromMe
		if len(message.Participant.Title) == 0 {
			message.Participant.Title = evt.Info.PushName
		}
	} else {
		if len(message.Chat.Title) == 0 && message.FromMe {
			message.Chat.Title = library.GetPhoneByWId(message.Chat.Id)
		}
	}

	// Process diferent message types
	HandleKnowingMessages(handler, message, evt.Message)

	// discard and return
	if message.Type == whatsapp.DiscardMessageType {
		JsonMsg := ToJson(evt)
		logentry.Debugf("debugging and ignoring an discard message: %s", JsonMsg)
		return
	}

	// unknown and continue
	if message.Type == whatsapp.UnknownMessageType {
		JsonMsg := ToJson(evt)
		logentry.Infof("debugging an unknown message :: %s", JsonMsg)
	}

	handler.Follow(message)
}

func ToJson(in interface{}) string {
	bytes, err := json.Marshal(in)
	if err == nil {
		return string(bytes)
	}
	return ""
}

//#endregion

/*
<summary>

	Follow throw internal handlers

</summary>
*/

// Append to cache handlers if exists, and then webhook
func (handler *WhatsmeowHandlers) Follow(message *whatsapp.WhatsappMessage) {
	if handler.WAHandlers != nil {

		// following to internal handlers
		go handler.WAHandlers.Message(message)

	} else {
		handler.GetLogger().Warn("no internal handler registered")
	}

	// testing, mark read function
	if handler.WhatsappOptionsExtended.ReadUpdate && !message.FromBroadcast() {
		go handler.MarkRead(message, types.ReceiptTypeRead)
	}
}

func (handler *WhatsmeowHandlers) MarkRead(message *whatsapp.WhatsappMessage, receipt types.ReceiptType) (err error) {
	logentry := handler.GetLogger()

	client := handler.Client
	ids := []string{message.Id}
	chatJID, err := types.ParseJID(message.Chat.Id)
	if err != nil {
		logentry.Errorf("error on mark read, parsing chat jid: %s", err.Error())
		return
	}

	var senderJID types.JID
	if message.Participant != nil {
		senderJID, err = types.ParseJID(message.Participant.Id)
		if err != nil {
			logentry.Errorf("error on mark read, parsing sender jid: %s", err.Error())
			return
		}
	}

	readtime := time.Now()
	err = client.MarkRead(ids, readtime, chatJID, senderJID, receipt)
	if err != nil {
		logentry.Errorf("error on mark read: %s", err.Error())
		return
	}

	logentry.Debugf("marked read chat id: %s, at: %v", message.Chat.Id, readtime)
	return
}

//#region EVENT CALL

func (handler *WhatsmeowHandlers) CallMessage(evt types.BasicCallMeta) {
	handler.GetLogger().Trace("event CallMessage !")

	message := &whatsapp.WhatsappMessage{Content: evt}

	// basic information
	message.Id = evt.CallID
	message.Timestamp = evt.Timestamp
	message.FromMe = false

	message.Chat = whatsapp.WhatsappChat{}
	chatID := fmt.Sprint(evt.From.User, "@", evt.From.Server)
	message.Chat.Id = chatID

	message.Type = whatsapp.CallMessageType

	if handler.WAHandlers != nil {

		// following to internal handlers
		go handler.WAHandlers.Message(message)
	}

	// should reject this call
	if !handler.HandleCalls() {
		_ = handler.RejectCall(evt)
	}
}

func (source *WhatsmeowHandlers) RejectCall(v types.BasicCallMeta) error {
	if source == nil {
		return fmt.Errorf("nil source handler")
	}

	var node = binary.Node{
		Tag: "call",
		Attrs: binary.Attrs{
			"to": v.From,
			"id": source.Client.GenerateMessageID(),
		},
		Content: []binary.Node{
			{
				Tag: "reject",
				Attrs: binary.Attrs{
					"call-id":      v.CallID,
					"call-creator": v.CallCreator,
					"count":        0,
				},
				Content: nil,
			},
		},
	}

	source.GetLogger().Infof("rejecting incoming call from: %s", v.From)
	return source.Client.DangerousInternals().SendNode(node)
}

//#endregion

//#region EVENT READ RECEIPT

func (handler *WhatsmeowHandlers) Receipt(evt events.Receipt) {
	handler.GetLogger().Trace("event Receipt !")

	message := &whatsapp.WhatsappMessage{Content: evt}
	message.Id = "readreceipt"

	// basic information
	message.Timestamp = evt.Timestamp
	message.FromMe = false

	message.Chat = whatsapp.WhatsappChat{}
	chatID := fmt.Sprint(evt.Chat.User, "@", evt.Chat.Server)
	message.Chat.Id = chatID

	message.Type = whatsapp.SystemMessageType

	// message ids comma separated
	message.Text = strings.Join(evt.MessageIDs, ",")

	if handler.WAHandlers != nil {

		// following to internal handlers
		go handler.WAHandlers.Receipt(message)
	}
}

//#endregion

//#region HANDLE LOGGED OUT EVENT

func (handler *WhatsmeowHandlers) OnLoggedOutEvent(evt events.LoggedOut) {
	reason := evt.Reason.String()
	handler.GetLogger().Tracef("logged out %s", reason)

	if handler.WAHandlers != nil {
		handler.WAHandlers.LoggedOut(reason)
	}

	message := &whatsapp.WhatsappMessage{
		Timestamp: time.Now().Truncate(time.Second),
		Type:      whatsapp.SystemMessageType,
		Id:        handler.Client.GenerateMessageID(),
		Chat:      whatsapp.WASYSTEMCHAT,
		Text:      reason,
	}

	handler.Follow(message)
	handler.UnRegister()
}

//#endregion
