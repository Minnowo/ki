import {ApiDownloadInit, ApiDownloadData, ApiDownloadDone} from '../api/api_download';
import {fmtProgress, sleep} from '../util';

type DlProps = {
    fileIdHex: string;
    filename: string;
    basicToken: string | null;
    setStatus: (s: string) => void;
};

enum DownloadStatus {
    Ok = 'ok',
    Abort = 'abort',
    TryNext = 'tryNext',
    Error = 'error',
}

export type DownloadResult =
    | {s: DownloadStatus.Ok}
    | {s: DownloadStatus.Abort}
    | {s: DownloadStatus.TryNext}
    | {s: DownloadStatus.Error; error: unknown};

// Only if available:
// https://developer.mozilla.org/en-US/docs/Web/API/Window/showOpenFilePicker
const downloadUsingPickerAPI = async ({fileIdHex, filename, basicToken, setStatus}: DlProps): Promise<DownloadResult> => {
    if (!window.showSaveFilePicker) {
        console.error('no save file picker api');
        return {s: DownloadStatus.TryNext};
    }

    let writable: FileSystemWritableFileStream;
    try {
        const handle = await window.showSaveFilePicker({suggestedName: filename});
        writable = await handle.createWritable();
    } catch (e) {
        console.error(e);
        if (!(e instanceof Error)) {
            return {s: DownloadStatus.Error, error: e};
        }
        if (e.name === 'AbortError') {
            return {s: DownloadStatus.Abort};
        }
        return {s: DownloadStatus.TryNext};
    }

    let sessionID: string;
    let totalSize: number;
    try {
        const init = await ApiDownloadInit(fileIdHex, basicToken);
        sessionID = init.session_id;
        totalSize = init.file_size;
    } catch (e) {
        console.error(e);
        return {s: DownloadStatus.Error, error: e};
    }

    const startTime = Date.now();
    const retryAmnt = 10;

    let partNum = 0;
    let retries = 0;
    let bytesRead = 0;
    try {
        while (bytesRead < totalSize) {
            setStatus(fmtProgress(bytesRead, totalSize, startTime));

            let resp: Response;
            try {
                resp = await ApiDownloadData(fileIdHex, sessionID);
                partNum++;
            } catch (e) {
                retries++;

                if (!(e instanceof Error)) {
                    setStatus(`Unexpected error: ${e}`);
                    await writable.abort().catch(() => {});
                    return {s: DownloadStatus.Error, error: e};
                }

                if (retries > retryAmnt) {
                    setStatus(`Max retries exceed`);
                    await writable.abort().catch(() => {});
                    return {s: DownloadStatus.Error, error: e};
                }

                setStatus(`Download error: ${e.name}: Waiting before retrying...`);

                await sleep(500 * retries);

                continue;
            }

            if (resp.status !== 206) {
                setStatus(`Unexpected status: ${resp.status}`);
                await writable.abort().catch(() => {});
                return {
                    s: DownloadStatus.Error,
                    error: new Error(`Unexpected status: ${resp.status}`),
                };
            }

            if (resp.body === null) {
                setStatus(`Unexpected null body`);
                await writable.abort().catch(() => {});
                return {
                    s: DownloadStatus.Error,
                    error: new Error(`Unexpected null body`),
                };
            }

            const reader = resp.body.getReader();

            while (true) {
                const {done, value} = await reader.read();

                if (done) {
                    break;
                }

                try {
                    await writable.write(value);
                } catch (e) {
                    if (e instanceof DOMException && e.name === 'AbortError') {
                        return {s: DownloadStatus.Abort};
                    }

                    setStatus(`Unexpected error writing file: ${e}`);
                    await writable.abort().catch(() => {});
                    return {s: DownloadStatus.Error, error: e};
                }

                bytesRead += value.length;

                setStatus(fmtProgress(bytesRead, totalSize, startTime));
            }
        }

        try {
            await writable.close();
        } catch (e) {
            console.warn(e);
            await writable.abort().catch(() => {});
            return {s: DownloadStatus.Error, error: e};
        }
    } finally {
        await ApiDownloadDone(fileIdHex, sessionID).catch(() => {});
    }

    return {s: DownloadStatus.Ok};
};

const downloadUsingServiceWorker = async ({fileIdHex, filename, basicToken, setStatus}: DlProps): Promise<DownloadResult> => {
    return {s: DownloadStatus.TryNext};
};

const downloadUsingInMemoryBlobs = async ({fileIdHex, filename, basicToken, setStatus}: DlProps): Promise<DownloadResult> => {
    return {s: DownloadStatus.TryNext};
};

const doChunkedDownload = async (fileIdHex: string, filename: string, hasPassword: boolean, setStatus: (s: string) => void) => {
    let basicToken = null;

    if (hasPassword) {
        const p = prompt('Enter file password:');

        if (p === null) {
            return;
        }

        basicToken = 'Basic ' + btoa('0:' + p);
    }

    const parms: DlProps = {
        fileIdHex: fileIdHex,
        filename: filename,
        basicToken: basicToken,
        setStatus: setStatus,
    };

    let attempt;

    attempt = await downloadUsingPickerAPI(parms);
    console.info(attempt);

    if (attempt.s === DownloadStatus.Ok) {
        return;
    }
    if (attempt.s === DownloadStatus.Abort) {
        return;
    }
    if (attempt.s === DownloadStatus.Error) {
        console.error(attempt.error);
        return;
    }

    attempt = await downloadUsingServiceWorker(parms);
    console.info(attempt);

    if (attempt.s === DownloadStatus.Ok) {
        return;
    }
    if (attempt.s === DownloadStatus.Abort) {
        return;
    }
    if (attempt.s === DownloadStatus.Error) {
        console.error(attempt.error);
        return;
    }

    attempt = await downloadUsingInMemoryBlobs(parms);
    console.info(attempt);

    return;
};

export const InitDownload = (mountId: string, fileIdHex: string, filename: string, hasPassword: boolean): void => {
    document.addEventListener('DOMContentLoaded', () => {
        console.info('mounting');
        const root = document.getElementById(mountId);

        if (!root) {
            return;
        }

        console.info('mounting2');
        const setStatus = (s: string) => {
            console.log(s);
        };

        const btn = document.createElement('button');
        btn.textContent = 'Download';

        btn.onclick = () => {
            doChunkedDownload(fileIdHex, filename, hasPassword, setStatus);
        };
        console.info('mounting3');

        root.appendChild(btn);
    });
};
