# bedrocktool
a minecraft bedrock proxy that can among other things save worlds from servers

<br/>

## downloads:
### [here](https://github.com/bedrock-tool/bedrocktool/releases)

<br/>

## issues:

if you find an issue or a crash, please report it by opening a github issue with a screenshot of the crash or issue, thanks

<br/>

```
Usage: bedrocktool <flags> <subcommand> <subcommand args>

Subcommands:
        capture          capture packets in a pcap file
        help             describe subcommands and their syntax
        list-realms      prints all realms you have access to
        merge            merge 2 or more worlds
        packs            download resource packs from a server
        realms-token     print xbl3.0 token for realms api
        skins            download all skins from players on a server
        skins-proxy      download skins from players on a server with proxy
        worlds           download a world from a server


Top-level flags (use "bedrocktool flags" for a full list):
  -debug=false: debug mode (enables extra logging useful for finding bugs)
  -dns=false: enable dns server for consoles (use this if you need to connect on a console)
  -preload=false: preload resourcepacks for proxy (use this if you need server resourcepacks while using the proxy)
```