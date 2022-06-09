# Whatsticker

<p align="left"><img src="assets/logo.jpg" alt="mythra" height="100px"></p>
A Whatsapp bot that turns pictures, small videos and gifs into stickers with the caption <b><u><a name="caption">stickerize</u></a></b>

[Chat with Whatsticker](https://wa.me/19293792260)


## Usage


### Simple Requirements

If you are not interested in running a copy of the project then feel free to use the whatsticker number provided. You can also add it to group chats and it will work the exact same way. There might be occasional downtime but except it gets blocked it should be pretty reliable

### Technical Requirements

 - [Go 1.17](https://go.dev/)
 - [Docker Compose](https://docs.docker.com/compose/install/)

### Generating Credentials

 - Download Go modules

   ```bash
   # Downloads go modules
   go mod tidy
   ```
 - Run `go run main.go` to show qr code for login on terminal

   ![Example QR Code](assets/qrcode.webp)

 - Link device as explained on [WhatsApp FAQ](https://faq.whatsapp.com/web/download-and-installation/how-to-link-a-device/)

 A folder `db/` will be automatically created containing sqlite db with credentials. No need to login over and over except logged out on main device. You can copy this folder to target machine and it still works

### Running The Bot

 - Ensure `db/examplestore.db` exists and run `docker-compose up`
 - Open any chat (Personal/Group) where the logged in number is present
 - Send media with [caption](#caption) and number should respond with sticker

  ![Example response](assets/example_video.mp4)

**WARNING**: `db/` folder on root will contain `examplestore.db` which docker-compose expects to load as a volume. Do not create a public image using this folder or commit to version control as it can be used to impersonate you

## CLI Flags

The following flags are available

- `-log-level` : To switch between log verbosity between `INFO` and `DEBUG`
- `-reply-to`  : Set to true if bot should quote original messages with reply
- `-sender`    : Respond to only this jid e.g `234XXXXXXXXXX`


## Flow

```
|___ worker                              # Container for converting media to stickers (scaled to handle load)
|    |__ metadata                        # Sets sticker exif info
|    |   |__ metadata.go
|    |   |__ raw.exif
|    |
|    |__ convert                         # gets media info off the convert queue and converts to webp and passes it to complete queue
|    |   |__ convert.go
|    |
|    |__ Dockerfile
|    |__ main.go
|
|___ master                              # Container that has whatsapp specific information (only one instance possible)
|    |__ handler                         # Run validation on whatsapp media type events and pass to convert queue
|    |   |__ handler.go
|    |   |__ image.go
|    |   |__ video.go
|    |
|    |___ task                           # takes info off the complete queue, uploads webP to whatsapp server and sends as sticker
|    |    |__ upload.go
|    |
|    |___ Dockerfile
|    |├── main.go                        # Login to WA client. sets up whatsapp event handler and subscribe to queues
|
|__  docker-compose                      # Runs redis for queues, volumes for media, spins multiple workers and a worker
|├── LICENSE
|└── README.md
```

## Limits/Issues

 - [ ] _Media sizes/length enforced by code_
 - [ ] _Some sticker results for videos are not animated (WebP size exceeds 1MB)_
 - [X] _Sometimes original media cannot be downloaded (especially for quoting older media messages with caption)_
 - [ ] _reply-to flag causes iOS users to be [incorrectly tagged](https://github.com/tulir/whatsmeow/issues/135)_

## License

This project is opened under the [MIT License](LICENSE) which allows very broad use for both academic and commercial purposes

## Credits

Library/Resource | Use
------- | -----
[tulir/whatsmeow](https://github.com/tulir/whatsmeow) | whatsmeow is a Go library for the WhatsApp web multidevice API.
[ffmpeg](https://ffmpeg.org) | A complete cross platform solution to record, convert and stream video (and audio).
[cwebp](https://developers.google.com/speed/webp/docs/cwebp) | Compress an image file into WebP file
[webpmux](https://developers.google.com/speed/webp/docs/webpmux) | Write exif file to set metadata on stickers

