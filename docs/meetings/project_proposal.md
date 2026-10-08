# Project Proposal Meeting Notes
- Finish up individual assigned parts of proposal
- Consider the VSCode live extension for pair programming + debugging sessions
- Gavin to setup webhooks to ping discord when certain repo events happen
- Eventually we will have CI, once we start coding stuff
- Justin to be in charge of submission tomorrow 10/8

## Block Diagram of FTP
![FTP Block Diagram](../images/FTP_Block_Diagram.drawio.png)
> FTP Block Diagram with Protocols, derived from Fig. 1 in [RFC 939](https://www.rfc-editor.org/info/rfc959/)


### Sequence Diagram
```mermaid
sequenceDiagram
    participant C as FTP Client
    participant S as FTP Server
    participant F as File System

    C->>S: Connect (TCP Port 21)
    S-->>C: Ready

    C->>S: USER / PASS
    S-->>C: 230 Login Successful

    C->>S: PASV
    S-->>C: 227 Data Port

    C->>S: Open Data Connection
    C->>S: RETR file.txt
    S->>F: Read File
    F-->>S: File Contents
    S-->>C: 150 Starting Transfer
    S->>C: Send File Data
    S-->>C: 226 Transfer Complete

    C->>S: QUIT
    S-->>C: 221 Goodbye
```
> Sequence Diagram: Covering basic client-server interaction

## Methodology
- implementation steps -> see minimuim implementation in RFC, prioritize these and there larger components!