# IRC User Modes

| User mode | Done | Description | Restrictions |
|:---------:|:----:|-------------|--------------|
| B | X | Marks you as being a bot. This will add a line to /WHOIS so people can easily recognize bots. | |
| d | X | Makes it so you can not receive channel PRIVMSG's, except for messages prefixed with a [set::channel-command-prefix](set::channel-command-prefix) character. Could be used by bots to reduce traffic so they only see !somecmd type of things. | |
| D | X | Makes it so you can not receive private messages (PMs) from anyone except IRCOps, servers and services. | |
| G | X | Swear filter: filters out all the "bad words" configured in the [Badword block](Badword-block) | |
| H | X | Hide IRCOp status. Regular users using /WHOIS or other commands will not see that you are an IRC Operator. | IRCOp-only |
| I | X | Hide idle time in /WHOIS. | see set block for more details: [set::hide-idle-time](set::hide-idle-time) |
| i | X | Makes you so called 'invisible'. A confusing term to mean that you're just hidden from /WHO and /NAMES if queried by someone outside the channel. Normally set by default through [set::modes-on-connect](set::modes-on-connect) and often by the users' IRC client as well. | |
| o | X | IRC Operator | Set by server |
| p | X | Hide channels you are in from /WHOIS, for extra privacy. | |
| q | X | Unkickable (only by U-lines, eg: services) | IRCOp-only (but not all) |
| r | X | Indicates this is a "registered nick" | Set by services |
| R | X | Only receive private messages from users who are "registered users" (authenticated by Services) | |
| S | X | User is a services bot (gives some extra protection) | Services-only |
| s | X | Server notices for IRCOps, see [Snomasks](Snomasks) | IRCOp-only |
| T | X | Prevents you from receiving CTCP's. | |
| t | X | Indicates you are using a /VHOST | Set by server upon /VHOST, /OPER, /^HOST, .. |
| W | X | Lets you see when people do a /WHOIS on you. | IRCOp-only |
| w | X | Can listen to wallops messages (/WALLOPS from IRCOps) | |
| x | X | Gives you a hidden / cloaked hostname. | |
| Z | X | Allows only users on a secure connection to send you private messages/notices/CTCPs. Conversely, you can't send any such messages to non-secure users either. | |
| z | X | Indicates you are connected via SSL/TLS | Set by server |
