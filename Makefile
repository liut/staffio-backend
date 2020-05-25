.SILENT :
.PHONY : dep vet main clean dist package
DATE := `date '+%Y%m%d'`

WITH_ENV = env `cat .env 2>/dev/null | xargs`
GO=$(shell which go)


all: vet

vet:
	echo "Checking ./..."
	$(GO) vet -all ./...


test-ldap: vet
	mkdir -p tests
	@$(WITH_ENV) $(GO) test -v -cover -coverprofile tests/cover_ldap.out ./ldap
	@$(GO) tool cover -html=tests/cover_ldap.out -o tests/cover_ldap.out.html

test-schema: vet
	mkdir -p tests
	@$(WITH_ENV) $(GO) test -v -cover -coverprofile tests/cover_schema.out ./schema
	@$(GO) tool cover -html=tests/cover_schema.out -o tests/cover_schema.out.html
