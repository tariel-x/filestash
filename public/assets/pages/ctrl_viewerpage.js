import { createElement, createRender } from "../lib/skeleton/index.js";
import rxjs, { effect } from "../lib/rx.js";
import { ApplicationError } from "../lib/error.js";
import { basename } from "../lib/path.js";
import { loadCSS } from "../helpers/loader.js";
import WithShell from "../components/decorator_shell_filemanager.js";
import { get as getConfig } from "../model/config.js";

import ctrlError from "./ctrl_error.js";
import { opener } from "./viewerpage/mimetype.js";
import { getCurrentPath } from "./viewerpage/common.js";

import "../components/breadcrumb.js";

const mime$ = getConfig().pipe(
    rxjs.map((config) => config.mime),
    rxjs.shareReplay(),
);

function loadModule(appName) {
    switch(appName) {
    case "editor":
        return import("./viewerpage/application_codemirror.js");
    case "pdf":
        return import("./viewerpage/application_pdf.js");
    case "image":
        return import("./viewerpage/application_image.js");
    case "download":
        return import("./viewerpage/application_downloader.js");
    case "form":
        return import("./viewerpage/application_form.js");
    case "audio":
        return import("./viewerpage/application_audio.js");
    case "video":
        return import("./viewerpage/application_video.js");
    case "ebook":
        return import("./viewerpage/application_ebook.js");
    case "appframe":
        return import("./viewerpage/application_iframe.js");
    default:
        throw new ApplicationError("Internal Error", `Unknown opener app "${appName}" at "${getCurrentPath()}"`);
    }
};

export default WithShell(async function(render) {
    const $page = createElement(`<div class="component_page_viewerpage"></div>`);
    render($page);

    effect(mime$.pipe(
        rxjs.map((mimes) => opener(basename(getCurrentPath()), mimes)),
        rxjs.mergeMap(([opener, options]) => rxjs.from(loadModule(opener)).pipe(
            rxjs.map((module) => module.default(createRender($page), options)),
        )),
        rxjs.catchError(ctrlError()),
    ));

})

export async function init() {
    return Promise.all([
        loadCSS(import.meta.url, "./ctrl_viewerpage.css"),
        loadCSS(import.meta.url, "../components/decorator_shell_filemanager.css"),
        mime$.pipe(
            rxjs.map((mimes) => opener(basename(getCurrentPath()), mimes)),
            rxjs.mergeMap(([opener]) => loadModule(opener)),
            rxjs.mergeMap((module) => typeof module.init === "function"? module.init() : rxjs.EMPTY),
            rxjs.catchError(() => rxjs.EMPTY),
        ).toPromise(),
    ]);
}
