export function opener(file = "", mimes) {
    const mime = getMimeType(file, mimes);
    const type = mime.split("/")[0];

    if (window.overrides && typeof window.overrides["xdg-open"] === "function") {
        const openerFromPlugin = window.overrides["xdg-open"](mime);
        if (openerFromPlugin !== null) {
            return openerFromPlugin;
        }
    }

    if (type === "text") {
        return ["editor", null];
    } else if (mime === "application/pdf") {
        return ["pdf", { mime }];
    } else if (type === "image") {
        return ["image", { mime }];
    } else if (["application/javascript", "application/xml", "application/json",
        "application/x-perl"].indexOf(mime) !== -1) {
        return ["editor", { mime }];
    } else if (["audio/wave", "audio/mp3", "audio/flac", "audio/ogg"].indexOf(mime) !== -1) {
        return ["audio", { mime }];
    } else if (mime === "application/x-form") {
        return ["form", { mime }];
    } else if (type === "video" || mime === "application/ogg") {
        return ["video", { mime }];
    } else if(["application/epub+zip"].indexOf(mime) !== -1) {
        return ["ebook", { mime }];
    } else if (type === "application") {
        return ["download", { mime }];
    }
    return ["editor", { mime }];
}

function getMimeType(file, mimes = {}) {
    return mimes[file.split(".")[1]] || "text/plain";
}
