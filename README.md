# Backup google contacts via the People api

A simple project to back up google contacts into [.vcf](https://tools.ietf.org/html/rfc6350#section-6.3.1) (vCard) format
This will download all your contacts and write them to a .vcf file.

It is a bit hacky and unpolished as well as **unfinished**.
Use at your own risk. Especially in production.

For this to work you need a OAuth client ID from google (`credentials.json`) from a
Google cloud project

It will ask you to login on the first run.
Open the link, login, copy code and paste it into the console.
