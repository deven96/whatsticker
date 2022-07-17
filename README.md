# Whatsticker

:warning: :warning: The live number is currently banned by WhatsApp so it no longer works, we are investigating ways to avoid ban, one of which may involve a hefty rewrite to use whatsapp official api :warning: :warning:

<p align="left"><img src="assets/logo.jpg" alt="mythra" height="100px"></p>
A Whatsapp bot that turns pictures, small videos and gifs into stickers with the caption <b><u><a name="caption">stickerize</u></a></b>


[Chat with Whatsticker](https://wa.me/19293792260)


## Usage

https://user-images.githubusercontent.com/23453888/172902050-7e039696-2b31-469f-8d39-c900b80fae4b.mp4

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

 A folder `master/db/` will be automatically created containing sqlite db with credentials. No need to login over and over except logged out on main device. You can copy this folder to target machine and it still works

### Running The Bot

 - Ensure `master/db/examplestore.db` exists and run `docker-compose up`
 - Open any chat (Personal/Group) where the logged in number is present
 - Send media with [caption](#caption) and number should respond with sticker


**WARNING**: `master/db/` folder will contain `examplestore.db` which docker-compose expects to load as a volume. Do not create a public image using this folder or commit to version control as it can be used to impersonate you

## CLI Flags

The following flags are available

- `-log-level` : To switch between log verbosity between `INFO` and `DEBUG`
- `-reply-to`  : Set to true if bot should quote original messages with reply
- `-sender`    : Respond to only this jid e.g `234XXXXXXXXXX`

## Architecture
![Arch Diagram](assets/arch-diag.png)

Open the [architecture](assets/arch-diag.drawio) on [draw.io](https://draw.io) 


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
[prometheus](https://github.com/prometheus/client_golang) | Live metrics of stickerization 
