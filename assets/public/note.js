"use strict";

function post(url, vals, cb) {
    const data = new FormData();
    for (const key in vals) {
        data.append(key, vals[key]);
    }
    const xhr = new XMLHttpRequest();
    xhr.open('POST', url)
    xhr.onreadystatechange = function() { if (xhr.readyState === XMLHttpRequest.DONE) return cb(xhr.status, xhr.responseText) };
    xhr.send(data);
}

