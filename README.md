# xyz
JS Runtime wrapped in a Websocket Server

---

### Message Structure
required fields for incoming, outgoing, or internal messages.
- `channel` ; unique string identifier for the channel the message is coming from
- `topic` ; unique string identifier for the topic of the message coming from the channel
- `message` ; object containing any additional data related to the channel topic
- *ex.*
    - ```json
        {
            "channel":"twitch",
            "topic":"follow",
            "message": {
                "username":"justSpike"
            }
        }
        ```

---

### Directory Structure
the directory containing scripts must be formated as follows.
- `{flag.path}/{channel}/{topic}/script.js`
- *ex.*
    - ```
        {flag.path}/twitch/follow/greet_new_follower.js
        ```

---

### JS Features
go linked functions and variables made available to js scripts.
- `e` ; variable storing the event that triggered the run
- `wait( timeout int )` ; pause execution of js script (milliseconds)
- `send.ws({ ... })` ; send message to the websocket connection
- `send.internal({ ... })` ;  send message internally to trigger other scripts

---

