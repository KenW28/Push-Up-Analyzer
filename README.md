# Push-Up-Analyzer
First attempt at a small personal passion project to gain experience in ML using MCUs. This project will use Arduino Nana 33 Sense REV2 and will hopefully be used to correctly capture the motion of push ups and properly count the repitions. 


app.js - behavior and data wiring (what happens when you do xyz)
index.html - Defines what is on the page (Titles, text, tables)
styles.css - the look and layout (Colors, fonts, spacing)

Server
go.mod - Tells Go this folder is a project 

handler_auth - 
db.go - Where persistence long-term DB lives 

*LOGIN INFO*

Never store plaintext passwords

Always store a bcrypt hash.

HTTPS only in production

If you use cookies for login sessions, sending them over HTTP is risky.

Production should be https:// with Secure cookies.

Use secure cookies

HttpOnly (JS can’t steal it)

SameSite=Lax (helps against CSRF)

Secure=true in production

Rate limit login

Prevent brute force password guessing.

Don’t leak whether a username exists

For login errors, return a generic “invalid credentials”.