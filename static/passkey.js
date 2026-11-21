// Passkeys (WebAuthn) for Jim.Tennis. Self-contained: no libraries and no
// requests to anywhere but this site.
//
// The page marks elements with data attributes:
//   [data-passkey-ui]           shown only when this browser can use passkeys here
//   [data-passkey-unsupported]  shown only when it cannot
//   [data-passkey-status]       where messages are written (role="alert")
//   [data-passkey-signin]       button: sign in with a passkey
//       data-redirect="/path"       where to go afterwards (the server validates it)
//       data-passkey-username="#id" input whose value is sent as the username
//   [data-passkey-add]          button: add a passkey to the signed-in account
//       data-passkey-name="#id"     input holding the name for the new passkey
//       data-next="/path"           where to go afterwards (default: reload)
//   [data-passkey-setup]        button (first-run setup): submit its form with
//                               the hidden field named by data-field set to "passkey"
//   [data-passkey-link="/account/link/..."]
//                               button (invite or recovery link page): create a
//                               passkey for the link's account and sign in
//       data-passkey-name="#id"     input holding the name for the new passkey
//       data-link-remove="#id"      checkbox: also remove the old sign-in methods
//   [data-passkey-reauth]       button (sign-in methods page): confirm it's you
//                               with a passkey before setting a password
//   details[data-passkey-fallback]  opened when this browser cannot use passkeys
(function () {
    'use strict';

    // --- base64url <-> bytes (the WebAuthn JSON forms use base64url) ---

    function bytesToBase64url(buffer) {
        var bytes = new Uint8Array(buffer);
        var binary = '';
        for (var i = 0; i < bytes.length; i++) {
            binary += String.fromCharCode(bytes[i]);
        }
        return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    }

    function base64urlToBytes(text) {
        var base64 = String(text).replace(/-/g, '+').replace(/_/g, '/');
        while (base64.length % 4 !== 0) {
            base64 += '=';
        }
        var binary = atob(base64);
        var bytes = new Uint8Array(binary.length);
        for (var i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
        }
        return bytes.buffer;
    }

    // --- server JSON -> what navigator.credentials wants ---

    function decodeDescriptors(list) {
        return (list || []).map(function (d) {
            var out = { type: d.type, id: base64urlToBytes(d.id) };
            if (d.transports) {
                out.transports = d.transports;
            }
            return out;
        });
    }

    function decodeCreationOptions(json) {
        var pk = Object.assign({}, json.publicKey);
        pk.challenge = base64urlToBytes(pk.challenge);
        pk.user = Object.assign({}, pk.user, { id: base64urlToBytes(pk.user.id) });
        pk.excludeCredentials = decodeDescriptors(pk.excludeCredentials);
        return { publicKey: pk };
    }

    function decodeRequestOptions(json) {
        var pk = Object.assign({}, json.publicKey);
        pk.challenge = base64urlToBytes(pk.challenge);
        if (pk.allowCredentials) {
            pk.allowCredentials = decodeDescriptors(pk.allowCredentials);
        }
        return { publicKey: pk };
    }

    // --- the browser's credential -> JSON for the server ---

    function encodeCredential(cred) {
        var r = cred.response;
        var response = { clientDataJSON: bytesToBase64url(r.clientDataJSON) };
        if (r.attestationObject) {
            response.attestationObject = bytesToBase64url(r.attestationObject);
            if (typeof r.getTransports === 'function') {
                response.transports = r.getTransports();
            }
        }
        if (r.authenticatorData) {
            response.authenticatorData = bytesToBase64url(r.authenticatorData);
        }
        if (r.signature) {
            response.signature = bytesToBase64url(r.signature);
        }
        if (r.userHandle && r.userHandle.byteLength > 0) {
            response.userHandle = bytesToBase64url(r.userHandle);
        }
        var out = {
            id: bytesToBase64url(cred.rawId),
            rawId: bytesToBase64url(cred.rawId),
            type: cred.type,
            response: response,
            // No extensions are requested, so none are reported.
            clientExtensionResults: {}
        };
        if (cred.authenticatorAttachment) {
            out.authenticatorAttachment = cred.authenticatorAttachment;
        }
        return out;
    }

    // --- talking to the server ---

    function PasskeyError(message) {
        this.message = message;
    }

    function postJSON(url, body) {
        return fetch(url, {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
            body: JSON.stringify(body === undefined ? {} : body)
        }).then(function (res) {
            var type = res.headers.get('Content-Type') || '';
            if (type.indexOf('application/json') !== 0) {
                // A page instead of JSON: the session ended and we were sent to sign in.
                throw new PasskeyError('Your session has ended. Reload the page and sign in again.');
            }
            return res.json().then(function (data) {
                if (!res.ok) {
                    throw new PasskeyError((data && data.error) || 'Something went wrong. Please try again.');
                }
                return data;
            });
        });
    }

    function describeError(err, adding) {
        if (err instanceof PasskeyError) {
            return err.message;
        }
        switch (err && err.name) {
        case 'NotAllowedError':
        case 'AbortError':
            return adding
                ? 'No passkey was added: the request was cancelled or timed out. Try again when you are ready.'
                : 'Passkey sign-in was cancelled or timed out. Try again, or sign in another way.';
        case 'InvalidStateError':
            return adding
                ? 'This device already has a passkey for this account.'
                : 'That passkey could not be used here.';
        case 'SecurityError':
            return 'Passkeys do not work at this web address. Open the site at its usual address and try again.';
        case 'NotSupportedError':
            return 'This device cannot create the kind of passkey this site needs.';
        default:
            return 'Something went wrong with the passkey. Please try again.';
        }
    }

    // --- page wiring ---

    function supported() {
        return typeof window.PublicKeyCredential === 'function' &&
            !!navigator.credentials &&
            typeof navigator.credentials.create === 'function' &&
            typeof navigator.credentials.get === 'function';
    }

    function each(selector, fn) {
        Array.prototype.forEach.call(document.querySelectorAll(selector), fn);
    }

    function setStatus(message) {
        each('[data-passkey-status]', function (el) {
            el.textContent = message || '';
            el.hidden = !message;
        });
    }

    function busy(button, on) {
        button.disabled = on;
        if (on) {
            button.setAttribute('aria-busy', 'true');
        } else {
            button.removeAttribute('aria-busy');
        }
    }

    function inputValue(selector) {
        var input = selector ? document.querySelector(selector) : null;
        return input ? input.value.trim() : '';
    }

    function signIn(button) {
        var usernameField = button.getAttribute('data-passkey-username');
        var body = {};
        if (usernameField) {
            body.username = inputValue(usernameField);
            if (!body.username) {
                setStatus('Type your username first.');
                return;
            }
        }
        setStatus('');
        busy(button, true);
        postJSON('/login/passkey/begin', body).then(function (options) {
            return navigator.credentials.get(decodeRequestOptions(options));
        }).then(function (cred) {
            var url = '/login/passkey/finish';
            var redirect = button.getAttribute('data-redirect');
            if (redirect) {
                url += '?redirect=' + encodeURIComponent(redirect);
            }
            return postJSON(url, encodeCredential(cred));
        }).then(function (result) {
            window.location.assign(result.redirect || '/');
        }).catch(function (err) {
            busy(button, false);
            setStatus(describeError(err, false));
        });
    }

    function addPasskey(button) {
        var name = inputValue(button.getAttribute('data-passkey-name'));
        setStatus('');
        busy(button, true);
        postJSON('/account/passkeys/begin').then(function (options) {
            return navigator.credentials.create(decodeCreationOptions(options));
        }).then(function (cred) {
            return postJSON('/account/passkeys/finish?name=' + encodeURIComponent(name), encodeCredential(cred));
        }).then(function () {
            window.location.assign(button.getAttribute('data-next') || '/account/signin-methods?done=added');
        }).catch(function (err) {
            busy(button, false);
            setStatus(describeError(err, true));
        });
    }

    // An invite or recovery link: the link's own address (which carries its
    // secret) is the base of the two endpoints. It is only ever sent to this
    // site, in the request path, exactly as when the page was opened.
    function linkPasskey(button) {
        var base = button.getAttribute('data-passkey-link');
        var name = inputValue(button.getAttribute('data-passkey-name'));
        var removeSelector = button.getAttribute('data-link-remove');
        var removeBox = removeSelector ? document.querySelector(removeSelector) : null;
        setStatus('');
        busy(button, true);
        postJSON(base + '/passkey/begin').then(function (options) {
            return navigator.credentials.create(decodeCreationOptions(options));
        }).then(function (cred) {
            var url = base + '/passkey/finish?name=' + encodeURIComponent(name);
            if (removeBox && removeBox.checked) {
                url += '&remove=1';
            }
            return postJSON(url, encodeCredential(cred));
        }).then(function (result) {
            window.location.replace(result.redirect || '/');
        }).catch(function (err) {
            busy(button, false);
            setStatus(describeError(err, true));
        });
    }

    // Confirm it's you with one of your own passkeys before changing your
    // password. The server keeps the confirmation (5 minutes, single use);
    // the page reloads to show the password form without the current password.
    function reauthPasskey(button) {
        setStatus('');
        busy(button, true);
        postJSON('/account/reauth/passkey/begin').then(function (options) {
            return navigator.credentials.get(decodeRequestOptions(options));
        }).then(function (cred) {
            return postJSON('/account/reauth/passkey/finish', encodeCredential(cred));
        }).then(function (result) {
            window.location.assign(result.redirect || '/account/signin-methods');
        }).catch(function (err) {
            busy(button, false);
            setStatus(describeError(err, false));
        });
    }

    function setupWithPasskey(button) {
        var form = button.form;
        var field = form && form.elements[button.getAttribute('data-field')];
        if (!form || !field) {
            return;
        }
        field.value = 'passkey';
        if (typeof form.requestSubmit === 'function') {
            form.requestSubmit();
        } else if (form.checkValidity()) {
            form.submit();
        } else {
            form.reportValidity();
        }
        // If validation stopped the submit, the plain button must not inherit the choice.
        setTimeout(function () { field.value = ''; }, 0);
    }

    function init() {
        var ok = supported();
        each('[data-passkey-ui]', function (el) { el.hidden = !ok; });
        each('[data-passkey-unsupported]', function (el) { el.hidden = ok; });
        if (!ok) {
            each('details[data-passkey-fallback]', function (el) { el.open = true; });
            return;
        }
        each('[data-passkey-link]', function (button) {
            button.addEventListener('click', function () { linkPasskey(button); });
        });
        each('[data-passkey-signin]', function (button) {
            button.addEventListener('click', function () { signIn(button); });
        });
        each('[data-passkey-add]', function (button) {
            button.addEventListener('click', function () { addPasskey(button); });
        });
        each('[data-passkey-reauth]', function (button) {
            button.addEventListener('click', function () { reauthPasskey(button); });
        });
        each('[data-passkey-setup]', function (button) {
            button.addEventListener('click', function () { setupWithPasskey(button); });
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
