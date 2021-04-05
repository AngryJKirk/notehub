"use strict";

function $(id) { return document.getElementById(id) }

function submitForm() {
    const id = $("id").value;
    const text = $("text").value;
    const name = $("name").value;
    const deletion = id !== "" && text === "";
    if (deletion && !confirm("Do you want to delete this note?")) {
        return;
    }
    const resp = post("/", {
        "id": id,
        "text": text,
        "name": name,
        "password": $("password").value
    }, function (status, responseRaw) {
        const response = JSON.parse(responseRaw);
        if (status < 400 && response.Success) {
            window.location.replace(deletion ? "/" : "/" + response.Payload)
        } else {
            $('feedback').innerHTML = status + ": " + response.Payload;
        }
    });
}
