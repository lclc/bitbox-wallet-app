/**
 * Copyright 2018 Shift Devices AG
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import i18n from '../i18n/i18n';
import { extConfig } from './config';
import { call } from './qttransport';

export const apiPort = extConfig('{{ API_PORT }}', '8082');
export const apiToken = extConfig('{{ API_TOKEN }}', '');

export function isTLS() {
    return document.URL.startsWith('https://');
}

export function apiURL(endpoint) {
    return (isTLS() ? 'https://' : 'http://') + 'localhost:' + apiPort + '/api/' + endpoint;
}

function handleError(endpoint) {
    return function(json) {
        return new Promise((resolve, reject) => {
            if (json && json.error) {
                if (json.error.indexOf('hidapi: unknown failure') !== -1) {
                    // Ignore device communication errors. Usually
                    // happens when unplugged during an operation, in
                    // which case the result does not matter.
                    return;
                }
                console.error('error from endpoint', endpoint, json);
                alert(i18n.t('genericError')); // eslint-disable-line no-alert
                reject(json.error);
                return;
            }
            resolve(json);
        });
    };
}

export function apiGet(endpoint) {
    if (typeof qt !== 'undefined') {
        return call(JSON.stringify({
            method: 'GET',
            endpoint,
        }));
    }
    return fetch(apiURL(endpoint), {
        method: 'GET'
    }).then(response => response.json()).then(handleError(endpoint));
}

export function apiPost(endpoint, body) {
    if (typeof qt !== 'undefined') {
        return call(JSON.stringify({
            method: 'POST',
            endpoint,
            body: JSON.stringify(body)
        }));
    }
    return fetch(apiURL(endpoint), {
        method: 'POST',
        body: JSON.stringify(body)
    }).then(response => response.json()).then(handleError(endpoint));
}
