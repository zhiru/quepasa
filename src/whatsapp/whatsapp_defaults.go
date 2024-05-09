package whatsapp

const WhatsappWebAppName = "QuePasa"

const WhatsappBroadcasts = false   // default broadcast messages option if none was specified
const WhatsappReadReceipts = false // default read receipt option if none was specified
const WhatsappCalls = true         // default calls option if none was specified
const WhatsappGroups = true        // default group messages option if none was specified
const WhatsappHistorySync = false  // default historysync option if none was specified

// Custom System name defined on start
var WhatsappWebAppSystem string

// Mime type for PTT Audio messages (default)
const WhatsappPTTMime = "audio/ogg; codecs=opus"

// Mime types that if converted will work as usual
var WhatsappMIMEAudioPTTCompatible = [...]string{"application/ogg", "audio/ogg", "video/ogg", "audio/wav", "audio/wave", "audio/x-wav"}

// Mime types for audio messages, tested 1º
var WhatsappMIMEAudio = [...]string{"audio/oga", "audio/ogx", "audio/x-mpeg-3", "audio/mpeg3", "audio/mpeg", "audio/mp4"}

// Mime types for video messages, tested 2º
var WhatsappMIMEVideo = [...]string{"video/mp4"}

// Mime types for image messages, tested 3º
var WhatsappMIMEImage = [...]string{"image/png", "image/jpeg"}

// Mime types for document messages, tested 4º
var WhatsappMIMEDocument = [...]string{
	"text/xml", "application/pdf",
	"application/ogg", "audio/ogg", "audio/wav", "audio/wave", "audio/x-wav", // not accepted anymore as audio msgs, but still compatible for convert to ptt
}

// global invalid file prefix
const InvalidFilePrefix = "invalid-"
