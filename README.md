# thumbgen

## Overview

thumbgen is a golang web-service for generating and uploading thumbnails of media files.


## Usage
### Running on local with go
```bash
go run main.go
```
### Running via docker
```bash
docker build -t thumbgen:v-local .
docker run -d -p 4499:4499 thumbgen:v-local
```

### Running via docker-compose
```bash
# note : check docker-compose.yaml and modify as required before running this
docker-compose up
```

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.