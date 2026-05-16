
Ki is a simple, self-hosted temporary file sharing service.

Uploaded files are automatically deleted after either:
 - a certain number of downloads, or
 - a certain period of time

depending on the settings chosen by the uploader.

## Features

- [x] File uploads and downloads
- [x] File checksums (`SHA512`, `SHA256`, `SHA1`, `MD5`)
- [x] File expiration by download count or time limit
- [x] No file size limit
- [x] Chunked uploads/downloads to bypass proy size limits
- [x] Works without Javascript *(except for files larger than the proxy request limit)*
- [x] Password-protected downloads
- [x] Server-side encryption
- [x] CloudFlare MitM warning
- [x] Support for SSL without a reverse proxy
- [ ] End-to-end encryption *(currently supported through provided Bash commands on the upload/download pages)*

## Usage

Before starting the server, create one or more users who are allowed to upload files:
```sh
ki registry create \
  -u minno -p minno_password \
  -u bob -p bob_password

# Outputs ./users.json
```

Then start the server:
```sh
ki run --master-secret 12345678
```

Then open http://localhost:9070


## Building

Requirements:
- Go 1.25 or later
- npm
- Make *(optional if you read the Makefile and do the steps manually)*

Build the project:
```sh
make download-tools generate build-site
```





