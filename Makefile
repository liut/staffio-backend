.SILENT :
.PHONY : dep vet main clean dist package
DATE := `date '+%Y%m%d'`

WITH_ENV = env `cat .env 2>/dev/null | xargs`


all: vet

dep:
	go install golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow

vet:
	echo "Checking ./..."
	go vet -vettool=$(which shadow) -atomic -bool -copylocks -nilfunc -printf -rangeloops -unreachable -unsafeptr -unusedresult ./...


test-ldap: vet
	mkdir -p tests
	@$(WITH_ENV) DEBUG=staffio:ldap go test -v -cover -coverprofile tests/cover_ldap.out ./ldap
	@go tool cover -html=tests/cover_ldap.out -o tests/cover_ldap.out.html
