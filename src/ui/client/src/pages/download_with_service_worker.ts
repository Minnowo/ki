import {SW_DL_PREFIX} from '../constants';
import {BuildBaseDownloadUI} from './download';

export const SetupServiceWorker = async (root: HTMLElement, fileId: string, hasPassword: boolean) => {

    const ddl = SW_DL_PREFIX + fileId;

    const {div, form, input, status} = BuildBaseDownloadUI(
        'Service Worker',
        'Stream the file as an http(s) download using a service worker.'
    );

    form.action = ddl;
    form.method = 'GET';
    form.target = '_blank';
    form.className = 'flex flex-col gap-1';
    form.setAttribute('rel', 'noopener noreferrer');

    if (!hasPassword) {
        input.remove();
    }

    let reg: ServiceWorkerRegistration;
    try {
        reg = await navigator.serviceWorker.register('/static/js/sw.js', {scope: SW_DL_PREFIX});
    } catch (e) {
        console.error('SW registration failed:', e);
        return false;
    }

    const sw = await (async()=> { 

        const sw = reg.active ?? reg.installing ?? reg.waiting;

        if (!sw) {
            return null; 
        }

        if (sw.state === 'activated') {
            return sw;
        }

        return await new Promise<ServiceWorker | null>((resolve) => {
            sw.addEventListener('statechange', function handler() {
                if (sw.state === 'activated') {
                    sw.removeEventListener('statechange', handler);
                    resolve(reg.active);
                }
            });
        });
    })();

    if (sw === null) {
        return false;
    }

    form.onsubmit = () => {

        sw.postMessage("ping");

        let interval = setInterval(() => {
            // prevent the service worker from dying during the download....
            try {
                sw.postMessage("ping");
            } catch(e) {
                clearInterval(interval);
            }
        }, 5000);

        return true;
    };

    let isDownloading = false;

    navigator.serviceWorker.addEventListener('message', (event) => {

        const data = event.data;

        if (data?.type === 'DOWNLOAD_PROGRESS') {

            for(let i = 0; i < data.downloads.length; i++) {

                const dl = data.downloads[i];

                if (dl[0] === fileId) {
                    isDownloading = true;
                    status.textContent = `${dl[1].downloaded} / ${dl[1].total}`;;
                }
            }
        }
    });

    window.addEventListener('beforeunload', (event) => {
        if (isDownloading) {
            event.preventDefault();
            return 'A download is in progress, if you close this page it might be truncated';
        }
    });

    root.appendChild(div);

    return true;
};
