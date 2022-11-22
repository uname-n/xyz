if (e.Message.run == true) {
    console.log("xyz.js")

    send.ws({
        "channel":"out",
        "topic":"out",
        "message":{}
    })

    send.internal({
        "channel":"custom_internal",
        "topic":"new_round",
        "message":{
            "winner": "turkey"
        }
    })

}