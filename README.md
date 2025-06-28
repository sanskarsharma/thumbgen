# thumbgen

## Overview

thumbgen is a golang web-service for generating and uploading thumbnails of media files.
Demo [live here](https://thumbgen.pohawithpeanuts.com)


## Usage
### Running on local with go
```bash
go run main.go
```
### Running via docker
```bash
docker build -t thumbgen:v0 .
docker run -d -p 4499:4499 thumbgen:v0
```

### Running via docker-compose
```bash
# note : check docker-compose.yaml and modify as required before running this
docker-compose up
```

### Deploying using Clouflare {Workers + Containers}

This deploys to your cloudflare account - make sure to edit/remove custom domain in [wrangler.jsonc](wrangler.jsonc) -> `routes` as needed. 

```bash
# install
npm install
npx wrangler deploy

```

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Please make sure to update tests as appropriate.