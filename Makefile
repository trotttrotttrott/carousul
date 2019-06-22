test:
	docker build -t carousul-test -f Dockerfile.test .
	docker-compose up -d
	docker-compose exec job \
		bash -c 'cd /job && go test'
