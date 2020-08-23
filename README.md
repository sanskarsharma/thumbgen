# thumbgen

## Overview

thumbgen is a golang web-service for generating and uploading thumbnails of media files.


## Usage
### Running locally
```bash
git clone https://github.com/sanskarsharma/thumbgen.git
cd thumbgen
go run main.go
```
### Running via docker
```bash
git clone https://github.com/sanskarsharma/thumbgen.git
cd thumbgen
docker build -t thumbgen:v-local .
docker run -d -p 2712:2712 thumbgen:v-local
```

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.