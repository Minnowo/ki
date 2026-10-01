import {ApiDownloadData, ApiDownloadDone, ApiDownloadInit} from '../api/api_download';
import {fmtProgress, sleep} from '../util';
import {BuildBaseDownloadUI} from './download';

export const SetupMemoryBlob = async (root: HTMLElement, fileId: string, filename: string, hasPassword: boolean) => {
    const {div, form, status, input} = BuildBaseDownloadUI(
        'In Memory',
        'Downloads the whole file into this tab\'s memory, then saves it once it has finished. ' +
            'It works in any browser with JavaScript, but the file has to fit in memory, so large files can crash the tab. ' +
            'If the file size is larger than your computer has memory, do not use this option.'
    );

    if (!hasPassword) {
        input.remove();
    }

    form.onsubmit = (event) => {
        event.preventDefault();

        const setStatus = (s: string) => (status.textContent = s);
        const token = hasPassword ? btoa(`0:${input!.value}`) : null;

        downloadWithInMemoryBlob(fileId, filename, token, setStatus)
            .then((r) => setStatus(r))
            .catch((e) => setStatus(`Error: ${e}`));
    };

    root.appendChild(div);

    return true;
};

const downloadWithInMemoryBlob = async (
    fileIdHex: string,
    filename: string,
    basicToken: string | null,
    setStatus: (s: string) => void
): Promise<string> => {
    let sessionID: string;
    let totalSize: number;
    try {
        const init = await ApiDownloadInit(fileIdHex, basicToken);
        sessionID = init.session_id;
        totalSize = init.file_size;
    } catch (e) {
        console.error(e);
        if (!(e instanceof Error)) return `Unexpected error: ${e}`;
        return e.message;
    }

    const startTime = Date.now();
    const retryAmnt = 10;

    let retries = 0;
    let bytesRead = 0;
    let chunks: Uint8Array[] = [];

    try {
        while (bytesRead < totalSize) {
            setStatus(fmtProgress(bytesRead, totalSize, startTime));

            let resp: Response;
            try {
                resp = await ApiDownloadData(fileIdHex, sessionID);
            } catch (e) {
                retries++;
                if (!(e instanceof Error)) return `Unexpected error: ${e}`;

                if (retries > retryAmnt) return `Aborted: max retries exceed`;

                setStatus(`Download error: ${e.name}: Waiting before retrying...`);
                await sleep(500 * retries);
                continue;
            }

            if (resp.status !== 206) return `Unexpected status: ${resp.status}`;
            if (!resp.body) return `Unexpected null body`;

            const reader = resp.body.getReader();
            while (true) {
                const {done, value} = await reader.read();
                if (done) break;
                if (value) {
                    chunks.push(value);
                    bytesRead += value.length;
                    setStatus(fmtProgress(bytesRead, totalSize, startTime));
                }
            }
        }
    } finally {
        await ApiDownloadDone(fileIdHex, sessionID).catch(() => {});
    }

    const blob = new Blob(chunks, {type: 'application/octet-stream'});

    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename; // set filename
    a.click();
    URL.revokeObjectURL(url);

    return 'Done';
};
