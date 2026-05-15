import {SetupMemoryBlob} from './download_with_memory_blob';
import {SetupPickerAPI} from './download_with_picker_api';
import {SetupServiceWorker} from './download_with_service_worker';

export const BuildBaseDownloadUI = (title: string, desc: string) => {
    const div = document.createElement('div');
    div.className = 'text-left';

    const label = document.createElement('h2');
    label.textContent = title;
    label.title = desc;
    div.appendChild(label);

    const status = document.createElement('p');
    div.appendChild(status);

    const form = document.createElement('form');
    form.className = 'flex flex-col gap-1 m-0';

    const input = document.createElement('input');
    input.type = 'password';
    input.name = 'p';
    input.placeholder = 'file password';
    input.title = 'Enter the password for this file';
    input.required = true;
    form.appendChild(input);

    const button = document.createElement('button');
    button.type = 'submit';
    button.textContent = 'Download';

    form.appendChild(button);
    div.appendChild(form);

    return {
        div: div,
        form: form,
        label: label,
        status: status,
        input: input,
        button: button,
    };
};

export const InitDownload = (mountId: string, fileIdHex: string, filename: string, hasPassword: boolean): void => {
    document.addEventListener('DOMContentLoaded', async () => {
        const root = document.getElementById(mountId);

        if (!root) {
            return;
        }

        const isPickerApiOk = await SetupPickerAPI(root, fileIdHex, filename, hasPassword);
        console.info(`picker api ok: ${isPickerApiOk}`);

        const isServiceWorkerOk = await SetupServiceWorker(root, fileIdHex, hasPassword);
        console.info(`service worker ok: ${isServiceWorkerOk}`);

        await SetupMemoryBlob(root, fileIdHex, filename, hasPassword);
    });
};
