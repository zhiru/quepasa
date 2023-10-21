module github.com/nocodeleaks/quepasa/main

replace github.com/nocodeleaks/quepasa/controllers => ./controllers

replace github.com/nocodeleaks/quepasa/library => ./library

replace github.com/nocodeleaks/quepasa/metrics => ./metrics

replace github.com/nocodeleaks/quepasa/models => ./models

replace github.com/nocodeleaks/quepasa/whatsapp => ./whatsapp

replace github.com/nocodeleaks/quepasa/whatsmeow => ./whatsmeow

go 1.19

require (
	github.com/joho/godotenv v1.5.1
	github.com/nocodeleaks/quepasa/controllers v0.0.0-00010101000000-000000000000
	github.com/nocodeleaks/quepasa/models v0.0.0-00010101000000-000000000000
	github.com/nocodeleaks/quepasa/whatsapp v0.0.0-00010101000000-000000000000
	github.com/nocodeleaks/quepasa/whatsmeow v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.9.3
)

require (
	filippo.io/edwards25519 v1.0.0 // indirect
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/go-chi/chi/v5 v5.0.10 // indirect
	github.com/go-chi/jwtauth v4.0.4+incompatible // indirect
	github.com/go-openapi/jsonpointer v0.20.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/spec v0.20.9 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/gosimple/slug v1.13.1 // indirect
	github.com/gosimple/unidecode v1.0.1 // indirect
	github.com/jmoiron/sqlx v1.3.5 // indirect
	github.com/joncalhoun/migrate v0.0.2 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/nbutton23/zxcvbn-go v0.0.0-20210217022336-fa2cb2858354 // indirect
	github.com/nocodeleaks/quepasa/library v0.0.0-00010101000000-000000000000 // indirect
	github.com/nocodeleaks/quepasa/metrics v0.0.0-00010101000000-000000000000 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.11.1 // indirect
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e // indirect
	github.com/swaggo/files v1.0.1 // indirect
	github.com/swaggo/http-swagger v1.3.4 // indirect
	github.com/swaggo/swag v1.16.2 // indirect
	go.mau.fi/libsignal v0.1.0 // indirect
	go.mau.fi/util v0.1.0 // indirect
	go.mau.fi/whatsmeow v0.0.0-20230916142552-a743fdc23bf1 // indirect
	golang.org/x/crypto v0.13.0 // indirect
	golang.org/x/net v0.15.0 // indirect
	golang.org/x/sys v0.12.0 // indirect
	golang.org/x/tools v0.13.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
