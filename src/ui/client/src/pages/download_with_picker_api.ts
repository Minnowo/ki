import { ApiDownloadData, ApiDownloadDone, ApiDownloadInit } from "../api/api_download";
import { fmtProgress, sleep } from "../util";
import { BuildBaseDownloadUI } from "./download";


export const SetupPickerAPI = async (root:HTMLElement, fileId: string, filename:string, hasPassword: boolean) => {

    if (!window.showSaveFilePicker) {
        console.error('no save file picker api');
        return false;
    }

    const { div, form, status, input } = BuildBaseDownloadUI(
        "Picker API",
        "Stream the file to disk using the showOpenFilePicker API (if avialable)."
    );

    if (!hasPassword) {
        input.remove();
    }

    form.onsubmit = (event) => {
        event.preventDefault();

        const setStatus = (s: string) => status.textContent = s;
        const token = (hasPassword) ? btoa(`0:${input!.value}`) : null;

        downloadWithPickerAPI(fileId, filename, token, setStatus)
            .then((r) => setStatus(r))
            .catch((e) => setStatus(`Error: ${e}`));

    };

    root.appendChild(div);

    return true;
};

const downloadWithPickerAPI = async (
    fileIdHex:string, 
    filename: string, 
    basicToken: string|null, 
    setStatus: (s:string)=>void): Promise<string> => {

    if (!window.showSaveFilePicker) {
        return 'Save file picker API unavailable';
    }

    let writable: FileSystemWritableFileStream;
    try {
        const handle = await window.showSaveFilePicker({suggestedName: filename});
        writable = await handle.createWritable();
    } catch (e) {
        console.error(e);
        if (!(e instanceof Error)) {
                    return `Unexpected error: ${e}`;
        }
        if (e.name === 'AbortError') {
            return 'Aborted';
        }
        return e.message;
    }

    let sessionID: string;
    let totalSize: number;
    try {
        const init = await ApiDownloadInit(fileIdHex, basicToken);
        sessionID = init.session_id;
        totalSize = init.file_size;
    } catch (e) {
        console.error(e);
        if (!(e instanceof Error)) {
                    return `Unexpected error: ${e}`;
        }
        return e.message;
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
                    await writable.abort().catch(() => {});
                    return `Unexpected error: ${e}`;
                }

                if (retries > retryAmnt) {
                    await writable.abort().catch(() => {});
                    return `Aborted: max retries exceed`;
                }

                setStatus(`Download error: ${e.name}: Waiting before retrying...`);

                await sleep(500 * retries);

                continue;
            }

            if (resp.status !== 206) {
                await writable.abort().catch(() => {});
                return `Unexpected status: ${resp.status}`;
            }

            if (resp.body === null) {
                await writable.abort().catch(() => {});
                return `Unexpected null body`;
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
                        return 'Aborted';
                    }

                    await writable.abort().catch(() => {});
                    return `Unexpected error writing file: ${e}`;
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
        }
    } finally {
        await ApiDownloadDone(fileIdHex, sessionID).catch(() => {});
    }

    return 'Done';
};
