package models

import (
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/google/uuid"
	library "github.com/nocodeleaks/quepasa/library"
	whatsapp "github.com/nocodeleaks/quepasa/whatsapp"
)

type QpWhatsappServer struct {
	*QpServer
	QpDataWebhooks

	// should auto reconnect, false for qrcode scanner
	Reconnect bool `json:"reconnect"`

	connection     whatsapp.IWhatsappConnection `json:"-"`
	syncConnection *sync.Mutex                  `json:"-"` // Objeto de sinaleiro para evitar chamadas simultâneas a este objeto
	syncMessages   *sync.Mutex                  `json:"-"` // Objeto de sinaleiro para evitar chamadas simultâneas a este objeto

	//Battery        *WhatsAppBateryStatus        `json:"battery,omitempty"`

	StartTime time.Time `json:"starttime,omitempty"`

	// log entry
	Logger *log.Entry `json:"-"`

	Handler *QPWhatsappHandlers `json:"-"`
	WebHook *QPWebhookHandler   `json:"-"`

	// Stop request token
	StopRequested bool                   `json:"-"`
	db            QpDataServersInterface `json:"-"`
}

// get default log entry, never nil
func (source *QpWhatsappServer) GetLogger() *log.Entry {
	return source.Logger
}

//#region IMPLEMENTING WHATSAPP OPTIONS INTERFACE

func (source *QpWhatsappServer) GetOptions() *whatsapp.WhatsappOptions {
	if source == nil {
		return nil
	}

	return &source.WhatsappOptions
}

func (source *QpWhatsappServer) SetOptions(options *whatsapp.WhatsappOptions) error {
	source.WhatsappOptions = *options
	return source.Save()
}

//#endregion

// Ensure default handler
func (server *QpWhatsappServer) HandlerEnsure() {
	if server.Handler == nil {
		handlerMessages := make(map[string]whatsapp.WhatsappMessage)
		handler := &QPWhatsappHandlers{
			server:       server,
			messages:     handlerMessages,
			sync:         &sync.Mutex{},
			syncRegister: &sync.Mutex{},
		}

		server.Handler = handler
	}
}

//region IMPLEMENT OF INTERFACE STATE RECOVERY

func (server *QpWhatsappServer) GetStatus() whatsapp.WhatsappConnectionState {
	if server.connection == nil {
		if server.Verified {
			return whatsapp.UnPrepared
		} else {
			return whatsapp.UnVerified
		}
	} else {
		if server.StopRequested {
			if server.connection != nil && !server.connection.IsInterfaceNil() && server.connection.IsConnected() {
				return whatsapp.Stopping
			} else {
				return whatsapp.Stopped
			}
		} else {
			state := server.connection.GetStatus()
			if state == whatsapp.Disconnected && !server.Verified {
				return whatsapp.UnVerified
			}
			return state
		}
	}
}

//endregion
//region IMPLEMENT OF INTERFACE QUEPASA SERVER

// Returns whatsapp controller id on E164
// Ex: 5521967609095
func (server QpWhatsappServer) GetWId() string {
	return server.QpServer.Wid
}

func (source *QpWhatsappServer) DownloadData(id string) ([]byte, error) {
	msg, err := source.Handler.GetMessage(id)
	if err != nil {
		return nil, err
	}

	source.GetLogger().Infof("downloading msg data %s", id)
	return source.connection.DownloadData(&msg)
}

/*
<summary>

	Download attachment from msg id, optional use cached data or not

</summary>
*/
func (source *QpWhatsappServer) Download(id string, cache bool) (att *whatsapp.WhatsappAttachment, err error) {
	msg, err := source.Handler.GetMessage(id)
	if err != nil {
		return
	}

	source.GetLogger().Infof("downloading msg %s, using cache: %v", id, cache)
	att, err = source.connection.Download(&msg, cache)
	if err != nil {
		return
	}

	return
}

func (source *QpWhatsappServer) RevokeByPrefix(id string) (errors []error) {
	messages := source.Handler.GetMessagesByPrefix(id)
	for _, msg := range messages {
		source.GetLogger().Infof("revoking msg by prefix %s", msg.Id)
		err := source.connection.Revoke(&msg)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return
}

func (source *QpWhatsappServer) Revoke(id string) (err error) {
	msg, err := source.Handler.GetMessage(id)
	if err != nil {
		return
	}

	source.GetLogger().Infof("revoking msg %s", id)
	return source.connection.Revoke(&msg)
}

//endregion

//#region WEBHOOKS

func (source *QpWhatsappServer) GetWebHook(url string) *QpWhatsappServerWebhook {
	for _, item := range source.Webhooks {
		if item.Url == url {
			return &QpWhatsappServerWebhook{
				QpWebhook: item,
				server:    source,
			}
		}
	}
	return nil
}

func (source *QpWhatsappServer) GetWebHooksByUrl(filter string) (out []*QpWebhook) {
	for _, element := range source.Webhooks {
		if strings.Contains(element.Url, filter) {
			out = append(out, element)
		}
	}
	return
}

// Ensure default webhook handler
func (server *QpWhatsappServer) WebHookEnsure() {
	if server.WebHook == nil {
		server.WebHook = &QPWebhookHandler{server}
	}
}

//#endregion

func (server *QpWhatsappServer) GetMessages(timestamp time.Time) (messages []whatsapp.WhatsappMessage) {
	if !timestamp.IsZero() && timestamp.Unix() > 0 {
		err := server.connection.HistorySync(timestamp)
		if err != nil {
			logger := server.GetLogger()
			logger.Warnf("error on requested history sync: %s", err.Error())
		}
	}
	messages = append(messages, server.Handler.GetMessages(timestamp)...)
	return
}

// Roda de forma assíncrona, não interessa o resultado ao chamador
// Inicia o processo de tentativas de conexão de um servidor individual
func (source *QpWhatsappServer) Initialize() {
	if source == nil {
		panic("nil server, code error")
	}

	source.GetLogger().Info("initializing whatsapp server ...")
	err := source.Start()
	if err != nil {
		source.GetLogger().Errorf("initializing server error: %s", err.Error())
	}
}

// Update underlying connection and ensure trivials
func (source *QpWhatsappServer) UpdateConnection(connection whatsapp.IWhatsappConnection) {

	if source.connection != nil && !source.connection.IsInterfaceNil() {
		source.connection.Dispose("UpdateConnection")
	}

	source.connection = connection
	if source.Handler == nil {
		source.GetLogger().Warn("creating handlers ?! not implemented yet")
	}

	source.connection.UpdateHandler(source.Handler)

	// Registrando webhook
	webhookDispatcher := &QPWebhookHandler{source}
	if !source.Handler.IsAttached() {
		source.Handler.Register(webhookDispatcher)
	}
}

func (source *QpWhatsappServer) EnsureUnderlying() (err error) {

	if len(source.Wid) > 0 && !source.Verified {
		err = fmt.Errorf("not verified")
		return
	}

	source.syncConnection.Lock()
	defer source.syncConnection.Unlock()

	// conectar dispositivo
	if source.connection == nil {
		logger := source.GetLogger()

		options := &whatsapp.WhatsappConnectionOptions{
			WhatsappOptions: &source.WhatsappOptions,
			Wid:             source.Wid,
			Reconnect:       true,
			LogEntry:        logger,
		}

		logger.Infof("trying to create new whatsapp connection, auto reconnect: %v ...", options.Reconnect)

		connection, err := NewConnection(options)
		if err != nil {
			waError, ok := err.(whatsapp.WhatsappError)
			if ok {
				if waError.Unauthorized() {
					source.MarkVerified(false)
				}
			}
			return err
		} else {
			source.connection = connection
		}
	}

	return
}

// called from service started, after retrieve servers from database
func (source *QpWhatsappServer) Start() (err error) {
	logger := source.GetLogger()

	logger.Info("starting whatsapp server")
	err = source.EnsureUnderlying()
	if err != nil {
		return
	}

	state := source.GetStatus()
	logger.Debugf("starting whatsapp server ... on %s state", state)

	if !IsValidToStart(state) {
		err = fmt.Errorf("trying to start a server on an invalid state :: %s", state)
		logger.Warnf(err.Error())
		return
	}

	// reset stop requested token
	source.StopRequested = false

	if !source.Handler.IsAttached() {

		// Registrando webhook
		source.Handler.Register(source.WebHook)
	}

	// Atualizando manipuladores de eventos
	source.connection.UpdateHandler(source.Handler)

	logger.Infof("requesting connection ...")
	err = source.connection.Connect()
	if err != nil {
		return source.StartConnectionError(err)
	}

	if !source.connection.IsConnected() {
		logger.Infof("requesting connection again ...")
		err = source.connection.Connect()
		if err != nil {
			return source.StartConnectionError(err)
		}
	}

	// If at this moment the connect is already logged, ensure a valid mark
	if source.connection.IsValid() {
		source.MarkVerified(true)
	}

	return
}

// called after success paring devices
func (source *QpWhatsappServer) EnsureReady() (err error) {
	logger := source.GetLogger()

	logger.Info("ensuring that whatsapp server is ready")
	err = source.EnsureUnderlying()
	if err != nil {
		logger.Errorf("error on ensure underlaying connection: %s", err.Error())
		return
	}

	// reset stop requested token
	source.StopRequested = false

	if !source.Handler.IsAttached() {
		logger.Info("attaching handlers")

		// Registrando webhook
		source.Handler.Register(source.WebHook)
	} else {
		logger.Debug("handlers already attached")
	}

	// Atualizando manipuladores de eventos
	source.connection.UpdateHandler(source.Handler)

	if !source.connection.IsConnected() {
		logger.Info("requesting connection ...")
		err = source.connection.Connect()
		if err != nil {
			return source.StartConnectionError(err)
		}
	} else {
		logger.Debug("already connected")
	}

	// If at this moment the connect is already logged, ensure a valid mark
	source.MarkVerified(true)

	return
}

// Process an error at start connection
func (source *QpWhatsappServer) StartConnectionError(err error) error {
	logger := source.GetLogger()

	source.Disconnect("StartConnectionError")
	source.Handler.Clear()

	if _, ok := err.(*whatsapp.UnAuthorizedError); ok {
		logger.Warningf("unauthorized, setting unverified")
		return source.MarkVerified(false)
	}

	logger.Errorf("error on starting whatsapp server connection: %s", err.Error())
	return err
}

func (source *QpWhatsappServer) Stop(cause string) (err error) {
	if !source.StopRequested {

		// setting token
		source.StopRequested = true

		// loggging properly
		logentry := source.GetLogger()
		logentry.Infof("stopping server: %s", cause)

		source.Disconnect("stop: " + cause)

		if source.Handler != nil {
			source.Handler.Clear()
		}
	}

	return
}

func (source *QpWhatsappServer) Restart() (err error) {
	err = source.Stop("restart")
	if err != nil {
		return
	}

	// wait 1 second before continue
	time.Sleep(1 * time.Second)

	logentry := source.GetLogger()
	logentry.Info("re-initializing whatsapp server ...")

	return source.Start()
}

// Somente usar em caso de não ser permitida a reconxão automática
func (source *QpWhatsappServer) Disconnect(cause string) {
	if source.connection != nil && !source.connection.IsInterfaceNil() {
		if source.connection.IsConnected() {
			logentry := source.GetLogger()
			logentry.Infof("disconnecting whatsapp server by: %s", cause)

			source.connection.Dispose(cause)
			source.connection = nil
		}
	}
}

// Retorna o titulo em cache (se houver) do id passado em parametro
func (server *QpWhatsappServer) GetChatTitle(wid string) string {
	if !server.connection.IsInterfaceNil() {
		return server.connection.GetChatTitle(wid)
	}
	return ""
}

// Usado para exibir os servidores/bots de cada usuario em suas respectivas telas
func (server *QpWhatsappServer) GetOwnerID() string {
	return server.User
}

//region QP BOT EXTENSIONS

// Check if the current connection state is valid for a start method
func IsValidToStart(status whatsapp.WhatsappConnectionState) bool {
	if status == whatsapp.Stopped {
		return true
	}
	if status == whatsapp.Stopping {
		return true
	}
	if status == whatsapp.Disconnected {
		return true
	}
	if status == whatsapp.Failed {
		return true
	}
	return false
}

func (server *QpWhatsappServer) GetWorking() bool {
	status := server.GetStatus()
	if status <= whatsapp.Stopped {
		return false
	} else if status == whatsapp.Disconnected {
		return false
	}
	return true
}

func (server *QpWhatsappServer) GetStatusString() string {
	return server.GetStatus().String()
}

func (server *QpWhatsappServer) ID() string {
	return server.Wid
}

// Traduz o Wid para um número de telefone em formato E164
func (server *QpWhatsappServer) GetNumber() string {
	return library.GetPhoneByWId(server.Wid)
}

func (server *QpWhatsappServer) GetTimestamp() time.Time {
	return server.Timestamp
}

func (server *QpWhatsappServer) GetStartedTime() time.Time {
	return server.StartTime
}

func (server *QpWhatsappServer) GetConnection() whatsapp.IWhatsappConnection {
	return server.connection
}

func (server *QpWhatsappServer) Toggle() (err error) {
	if !server.GetWorking() {
		err = server.Start()
	} else {
		err = server.Stop("toggling")
	}
	return
}

func (server *QpWhatsappServer) IsDevelopmentGlobal() bool {
	switch ENV.LogLevel() {
	case "debug", "trace":
		return true
	default:
		return false
	}
}

/*
<summary>

	Set a new random Guid token for whatsapp server bot

</summary>
*/
func (server *QpWhatsappServer) CycleToken() (err error) {
	value := uuid.New().String()
	return server.UpdateToken(value)
}

/*
<summary>

	Set a specific not empty token for whatsapp server bot

</summary>
*/
func (source *QpWhatsappServer) UpdateToken(value string) (err error) {
	if len(value) == 0 {
		err = fmt.Errorf("empty token")
		return
	}

	err = source.UpdateToken(value)
	if err != nil {
		return
	}

	source.GetLogger().Infof("updating token: %v", value)
	return
}

/*
<summary>

	Get current token for whatsapp server bot

</summary>
*/
func (server *QpWhatsappServer) GetToken() string {
	return server.Token
}

/*
<summary>

	Save changes on database

</summary>
*/
func (source *QpWhatsappServer) Save() (err error) {
	logger := source.GetLogger()

	logger.Infof("saving server info: %+v", source)
	ok, err := source.db.Exists(source.Token)
	if err != nil {
		log.Errorf("error on checking existent server: %s", err.Error())
		return
	}

	// updating timestamp
	source.Timestamp = time.Now().UTC()

	if ok {
		logger.Debugf("updating server info: %+v", source)
		return source.db.Update(source.QpServer)
	} else {
		logger.Debugf("adding server info: %+v", source)
		return source.db.Add(source.QpServer)
	}
}

func (server *QpWhatsappServer) MarkVerified(value bool) (err error) {
	if server.Verified != value {
		server.Verified = value
		return server.Save()
	}
	return nil
}

func (source *QpWhatsappServer) ToggleDevel() (handle bool, err error) {
	source.Devel = !source.Devel

	logger := source.GetLogger()
	if source.Devel {
		logger.Logger.SetLevel(log.DebugLevel)
	} else {
		logger.Logger.SetLevel(log.InfoLevel)
	}

	return source.Devel, source.Save()
}

//endregion

// delete this whatsapp server and underlaying connection
func (server *QpWhatsappServer) Delete() (err error) {
	if server.connection != nil {
		err = server.connection.Delete()
		if err != nil {
			return
		}

		server.connection = nil
	}

	return server.db.Delete(server.Token)
}

//endregion
//#region SEND

// Default send message method
func (source *QpWhatsappServer) SendMessage(msg *whatsapp.WhatsappMessage) (response whatsapp.IWhatsappSendResponse, err error) {
	logger := source.GetLogger()
	logger.Debugf("sending msg to: %s", msg.Chat.Id)

	// leading with wrongs digit 9
	if ENV.ShouldRemoveDigit9() {

		phone, _ := library.ExtractPhoneIfValid(msg.Chat.Id)
		if len(phone) > 0 {
			phoneWithout9, _ := library.RemoveDigit9IfElegible(phone)
			if len(phoneWithout9) > 0 {
				valids, err := source.connection.IsOnWhatsApp(phone, phoneWithout9)
				if err != nil {
					return nil, err
				}

				for _, valid := range valids {
					logger.Debugf("found valid destination: %s", valid)
					msg.Chat.Id = valid
					break
				}
			}
		}
	}

	if msg.HasAttachment() {
		if len(msg.Text) > 0 {

			// Overriding filename with caption text if IMAGE or VIDEO
			if msg.Type == whatsapp.ImageMessageType || msg.Type == whatsapp.VideoMessageType {
				msg.Attachment.FileName = msg.Text
			} else {

				// Copying and send text before file
				textMsg := *msg
				textMsg.Type = whatsapp.TextMessageType
				textMsg.Attachment = nil
				response, err = source.connection.Send(&textMsg)
				if err != nil {
					return
				} else {
					source.Handler.Message(&textMsg)
				}
			}
		}
	}

	// sending default msg
	response, err = source.connection.Send(msg)
	if err == nil {
		source.Handler.Message(msg)
	}
	return
}

//#endregion
//#region PROFILE PICTURE

func (source *QpWhatsappServer) GetProfilePicture(wid string, knowingId string) (picture *whatsapp.WhatsappProfilePicture, err error) {
	logger := source.GetLogger()
	logger.Debugf("getting info about profile picture for: %s, with id: %s", wid, knowingId)

	return source.connection.GetProfilePicture(wid, knowingId)
}

//#endregion
//#region GROUP INVITE LINK

func (server *QpWhatsappServer) GetInvite(groupId string) (link string, err error) {
	return server.connection.GetInvite(groupId)
}

//#endregion
