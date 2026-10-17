(function () {
    "use strict";

    // HTMX normally encodes form parameters as application/x-www-form-urlencoded.
    // Authentication endpoints use the same JSON contract as the public API, so
    // opt these forms into a small local encoder instead of sending credentials
    // as a raw form body.
    if (!window.htmx || typeof window.htmx.defineExtension !== "function") {
        return;
    }

    window.htmx.defineExtension("json-enc", {
        onEvent: function (name, event) {
            if (name === "htmx:configRequest") {
                event.detail.headers["Content-Type"] = "application/json";
            }
        },

        encodeParameters: function (xhr, parameters) {
            xhr.overrideMimeType("application/json");
            return JSON.stringify(parameters);
        }
    });
})();
