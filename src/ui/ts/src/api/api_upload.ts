import {UploadSessionDataResponse, UploadSessionDoneResponse, UploadSessionInitResponse} from './types';
import {apiFetch, authBearer, fetchJson} from './util';

export const ApiUploadInit = (body: FormData) => {
    return fetchJson<UploadSessionInitResponse>('/api/ul/s/init', {
        method: 'POST',
        body: body,
    });
};

export const ApiUploadData = (sessionId: string, chunk: Blob, progress: (loaded: number) => void) => {
    return new Promise<UploadSessionDataResponse>((resolve, reject) => {
        const xhr = new XMLHttpRequest();

        xhr.open('POST', `/api/ul/s/data`, true);
        xhr.responseType = 'json';

        xhr.setRequestHeader('Content-Type', 'application/octet-stream');
        xhr.setRequestHeader('Authorization', authBearer(sessionId));
        xhr.setRequestHeader('X-Requested-With', 'js-form');
        xhr.upload.onprogress = (e) => {
            if (e.lengthComputable) {
                progress(e.loaded);
            }
        };
        xhr.onload = () => {
            if (xhr.status === 200) {
                resolve(xhr.response);
            } else {
                const errorData = xhr.response ?? xhr.responseText ?? `HTTP ${xhr.status}`;

                reject(new Error(errorData));
            }
        };
        xhr.onerror = () => reject(new Error('Network error'));
        xhr.onabort = () => reject(new Error('aborted'));
        xhr.send(chunk);
    });
};

export const ApiUploadDone = (sessionId: string) => {
    return fetchJson<UploadSessionDoneResponse>(`/api/ul/s/done`, {
        method: 'POST',
        headers: {
            Authorization: authBearer(sessionId),
        },
    });
};

export const ApiUploadAbort = (sessionId: string) => {
    return apiFetch(`/api/ul/s/abort`, {
        method: 'POST',
        headers: {
            Authorization: authBearer(sessionId),
        },
    });
};
