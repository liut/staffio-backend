module github.com/liut/staffio-backend

go 1.21

require (
	github.com/go-ldap/ldap/v3 v3.4.10
	github.com/stretchr/testify v1.8.1
	go.uber.org/zap v1.27.0
)

require (
	github.com/Azure/go-ntlmssp v0.0.0-20221128193559-754e69321358 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.7 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract [v1.0.0, v0.2.5]
