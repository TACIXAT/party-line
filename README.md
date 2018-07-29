# Party Line

```
░▓▓▓▓  ░▓▓▓  ░▓▓▓ ░▓▓▓▓▓░▓  ░▓    ░▓    ░▓▓▓▓▓░▓  ░▓░▓▓▓▓▓ ░▓
░▓  ░▓░▓  ░▓░▓  ░▓  ░▓  ░▓  ░▓    ░▓      ░▓  ░▓▓ ░▓░▓     ░▓
░▓  ░▓░▓  ░▓░▓  ░▓  ░▓   ░▓░▓     ░▓      ░▓  ░▓░▓░▓░▓     ░▓
░▓▓▓▓ ░▓▓▓▓▓░▓▓▓▓   ░▓    ░▓      ░▓      ░▓  ░▓ ░▓▓░▓▓▓▓  ░▓
░▓    ░▓  ░▓░▓  ░▓  ░▓    ░▓      ░▓      ░▓  ░▓  ░▓░▓     ░▓
░▓    ░▓  ░▓░▓  ░▓  ░▓    ░▓      ░▓      ░▓  ░▓  ░▓░▓       
░▓    ░▓  ░▓░▓  ░▓  ░▓    ░▓      ░▓▓▓▓▓░▓▓▓▓▓░▓  ░▓░▓▓▓▓▓ ░▓
```

Party Line is a decentralized peer to peer chat app with message integrity and end to end encryption. 

Main Line is the primary chat and control channel for the application. Things aren't encrypted in Main Line because the chat is global. Messages are signed and verifiable for [integrity porpoises](https://upload.wikimedia.org/wikipedia/commons/8/82/Delfinekko.gif).

Party Lines are sub chats, established between a set of peers. These chats have end to end encryption and file transfer capabilities. 

## Getting Started

```bash
git clone https://github.com/douggard/party-line.git
cd party-line
go run *.go
```

Or download the prebuilt binaries from /bin (links on http://party-line.lol).

## Usage

When you start you'll see an ID that looks like `ip_addr/port/hex_id`. You need to find someone else's ID in order to bootstrap to them (or you can send yours to them for them to bootstrap to you.) You do this in the client with the `/bs` command.

```
/bs 192.5.18.184/1235/92b45c3818331430628fbe393d43f3c07f529c3a8fb70f67bb543d78d5320223
```

Here are the other commands.

```
this is probably wildly out of date...
/bs [bootstrap info]
    show bs info (no arg) or bootstrap to a peer
/start <party_name>
    start a party (name limit 8 characters)
/invite <party_id> <user_id>
    invite a user to a party (partial ids ok)
/accept <party_id>
    accept an invite (partial ids ok)
/list [parties|invites]
    list parties, invites, or both
/send <party_id> msg
    send message to party (partial id ok)
/leave <party_id>
    leaves the party (partial id ok)
/show [all|mainline|party_id]
    display and send to selected channel (partial id ok)
/clear
    clear chat log
/ids <size>
    change id display size (hex)
/help
    display this
/quit
    we'll miss you, but it was fun while you were 
```

## FAQQIES

**Why should I not trust Main Line?**

It's full of people like you. 

**Are Party Lines secure?**

Party Lines rely on you trusting all individuals participating in that Party Line. The messages are end to end encrypted, because fuck the snoopers, but anyone in that channel could log it or invite someone else who isn't trusted. Other than that they're pretty legit. Speak freely about your plots and schemes, send noodles, share your rarest memes.

**When's the ICO?**

Digital "assets" that lack monetary policy will never be realized as mainstream currencies. 

**B-b-but muh Bitcoin debit card?**

That's just a passthrough gimmick for spending money in USD. You also pay taxes in USD. Thanks for your continued support of the one true currency.

**Is this the blockchain?**

Yea, fuck it, why not? We use a chain of blocks and hashes to verify the file pieces in the file packs.

**Is this Tor?**

No. While your messages will often route through other users, routing tables are public which contain your ID, IP, and port. Since you sign all your messages with your key and ID, it is not unrealistic that people could figure out which IP is posting which messages. 

**Is this just a straight ripoff of 9gridchan?**

Yes.

**Will you accept my pull request for an Electron front-end?**

Yes, thank you. Strategic decisions like this will help us along our path to a billion dollar IPO. Who needs RAM when you have servants?

## Thanksies

@mabynogy

@morla10111

mycroftiv
