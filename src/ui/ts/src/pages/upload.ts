import {ApiUploadAbort, ApiUploadInit, ApiUploadData, ApiUploadDone} from '../api/api_upload';
import {fmtProgress} from '../util';

const makeProgressUI = () => {
    const cont = document.createElement('div');
    cont.style.display = 'flex';
    cont.style.alignItems = 'center';
    cont.style.justifyContent = 'center';
    cont.style.gap = '10px';

    const prog = document.createElement('div');
    prog.style.whiteSpace = 'pre';
    prog.style.fontFamily = 'monospace';

    const cancel = document.createElement('button');
    cancel.type = 'button';
    cancel.textContent = 'Cancel';
    cancel.style.cursor = 'pointer';

    cont.appendChild(prog);
    cont.appendChild(cancel);

    return {cont, prog, cancel};
};

function doSingleUpload(form: HTMLFormElement) {
    const {cont, prog, cancel} = makeProgressUI();
    const xhr = new XMLHttpRequest();
    const startTime = Date.now();

    cancel.onclick = () => {
        xhr.abort();
        cont.remove();
    };

    xhr.open(form.method, form.action, true);
    xhr.setRequestHeader('X-Requested-With', 'js-form');

    xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
            prog.textContent = fmtProgress(e.loaded, e.total, startTime);
        }
    };

    xhr.onload = () => {
        const location = xhr.getResponseHeader('Location');
        if (location && xhr.status === 200) {
            window.location.href = location;
        } else {
            cont.remove();
            alert('Upload failed: ' + xhr.status);
        }
    };

    xhr.onerror = () => {
        cont.remove();
        alert('Network error during upload.');
    };

    xhr.send(new FormData(form));
}

async function doChunkedUpload(form: HTMLFormElement, fileField: HTMLInputElement, file: File, setStatus: (s: string) => void) {
    const {cont, prog, cancel} = makeProgressUI();
    let uploadId: string | null = null;
    let sessiondID: string | null = null;
    let aborted = false;
    let currentXHR = null;

    cancel.onclick = () => {
        aborted = true;
        if (sessiondID) {
            ApiUploadAbort(sessiondID).catch(() => {});
        }
        cont.remove();
    };

    prog.textContent = 'Starting upload...';

    const beginData = new FormData(form);
    beginData.delete(fileField.name);
    beginData.set('filename', file.name);

    // let uploadId : string;
    let maxChunkSize: number;
    try {
        const init = await ApiUploadInit(beginData);
        uploadId = init.upload_id;
        maxChunkSize = init.max_chunk_size;
    } catch (e) {
        cont.remove();
        alert('Network error during upload.');
        return;
    }

    // 2. Send chunks with XHR so xhr.upload.onprogress fires within each chunk.
    let offset = 0;
    const total = file.size;
    const startTime = Date.now();

    while (offset < total) {
        if (aborted) return;

        const chunk = file.slice(offset, offset + maxChunkSize);
        const chunkOffset = offset; // bytes fully uploaded before this chunk

        offset += chunk.size;

        try {
            await ApiUploadData(uploadId, chunk, (n: number) => {
                setStatus(fmtProgress(chunkOffset + n, total, startTime));
            });
        } catch (e) {
            return;
        }
    }

    if (aborted) return;

    // 3. Complete (no body, fetch is fine).
    prog.textContent = 'Finalising...';

    let file_id: string;
    try {
        const done = await ApiUploadDone(uploadId);
        file_id = done.file_id;
    } catch (e) {
        cont.remove();
        alert('Network error during upload.');
        return;
    }

    if (location) {
        setTimeout(() => {
            window.location.href = `/download/${file_id}`;
        }, 2000);
    }
}

export const InitUpload = (mountId: string, formId: string, maxUploadSize: number, chunkSize: number): void => {
    document.addEventListener('DOMContentLoaded', () => {
        const root = document.getElementById(mountId);

        if (!root) {
            return;
        }

        const form: HTMLFormElement | null = document.getElementById(formId) as HTMLFormElement;

        if (!form) {
            return;
        }

        const setStatus = (s: string) => {
            console.log(s);
        };

        form.addEventListener('submit', (event) => {
            event.preventDefault();

            const fileField = form.querySelector<HTMLInputElement>('input[type="file"]');

            if (!fileField) {
                setStatus('Could not find file field.');
                return;
            }

            if (!fileField.files) {
                setStatus('No files selected');
                return;
            }

            const file = fileField.files[0];

            if (file.size > maxUploadSize) {
                setStatus('File size is too large');
            } else if (file.size > chunkSize) {
                doChunkedUpload(form, fileField, file);
            } else {
                doSingleUpload(form);
            }
        });
    });
};
