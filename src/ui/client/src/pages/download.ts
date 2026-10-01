import {SetupMemoryBlob} from './download_with_memory_blob';
import {SetupPickerAPI} from './download_with_picker_api';
import {SetupServiceWorker} from './download_with_service_worker';

// link builds an external link that opens in a new tab, for use in help text.
export const link = (href: string, text: string) => {
    const a = document.createElement('a');
    a.href = href;
    a.textContent = text;
    a.target = '_blank';
    a.rel = 'noopener noreferrer';
    return a;
};

// helpText is plain text, or a mix of text and elements (e.g. a link) to show in order.
export const BuildBaseDownloadUI = (title: string, helpText: string | (string | Node)[]) => {
    const div = document.createElement('div');
    div.className = 'surface-2 flex flex-col gap-2';

    const details = document.createElement('details');
    details.className = 'flex flex-col gap-2';

    const summary = document.createElement('summary');
    summary.className = 'flex items-baseline gap-2 cursor-pointer';

    const label = document.createElement('h3');
    label.textContent = title;
    summary.appendChild(label);

    const hint = document.createElement('small');
    hint.textContent = '(click for help)';
    summary.appendChild(hint);

    const help = document.createElement('p');
    help.append(...(typeof helpText === 'string' ? [helpText] : helpText));

    details.appendChild(summary);
    details.appendChild(help);
    div.appendChild(details);

    const status = document.createElement('small');
    status.className = 'empty:hidden';
    div.appendChild(status);

    const form = document.createElement('form');
    form.className = 'flex flex-col gap-2';

    const input = document.createElement('input');
    input.type = 'password';
    input.name = 'p';
    input.placeholder = 'File password';
    input.setAttribute('aria-label', 'File password');
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
