docker-build:
	docker build -t carousul-test -f Dockerfile.test .

test: docker-build
	docker-compose up -d
	docker-compose exec job \
		bash -c 'cd /job && go test'

go-build: docker-build
	docker run --rm \
		-e VERSION=`cat VERSION` \
		-v ${PWD}/vendor:/go/src \
		-v ${PWD}:/job \
		carousul-test \
		bash -c 'cd /job && GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -o dist/carousul-$$VERSION'
