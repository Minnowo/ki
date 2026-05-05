import { SW_DL_PREFIX } from "../constants";
import { BuildBaseDownloadUI } from "./download";

export const SetupServiceWorker = async (root:HTMLElement, fileId: string, hasPassword: boolean) => {

    let reg: ServiceWorkerRegistration;
    try {
        reg = await navigator.serviceWorker.register('/static/js/sw.js', { scope: SW_DL_PREFIX });
    } catch (e) {
        console.error('SW registration failed:', e);
        return false;
    }

    const sw = reg.active ?? reg.installing ?? reg.waiting;

    if (sw && sw.state !== 'activated') {
        await new Promise<void>((resolve) => {
            sw.addEventListener('statechange', function handler() {
                if (sw.state === 'activated') {
                    sw.removeEventListener('statechange', handler);
                    resolve();
                }
            });
        });
    }

    const ddl = SW_DL_PREFIX + fileId;

    const { div, form, input } = BuildBaseDownloadUI(
        "Service Worker",
        "Stream the file as an http(s) download using a service worker."
    );

    form.action = ddl;
    form.method = "GET";
    form.target = "_blank";
    form.className = "flex flex-col gap-1";
    form.setAttribute("rel", "noopener noreferrer");

    if (!hasPassword) {
        input.remove();
    }

    root.appendChild(div);

    return true;
};

