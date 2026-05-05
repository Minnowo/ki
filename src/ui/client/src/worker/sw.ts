import { ApiDownloadData, ApiDownloadDone, ApiDownloadInit } from "../api/api_download";
import { SW_DL_PREFIX } from "../constants";
import { sleep } from "../util";

export {};

declare var self: ServiceWorkerGlobalScope;

const handleDownloadRequest = (fileIdHex: string, token: string|null): Promise<Response> => {

    return (async (): Promise<Response> => {

        console.info("beginning chunked download");
        
        let sessionId: string;
        let filename: string;
        let fileSize: number;
        try{
            const init = await ApiDownloadInit(fileIdHex, token);
            sessionId = init.session_id;
            filename = init.filename;
            fileSize = init.file_size;
        } catch(e) {
            return new Response('Failed to initialize download session', {status: 400});
        }

        const signalDone = () => ApiDownloadDone(fileIdHex, sessionId).catch(() => {})

        const retryAmnt = 10;
        let bytesRead = 0;
        let retries = 0;
        let chunk = 0;

        const stream = new ReadableStream<Uint8Array>({
            async cancel(reason: string) {
                console.error(`Canceled download: ${reason}`);
                await signalDone();
            },
            async pull(controller: ReadableStreamDefaultController) {

                while (bytesRead < fileSize) {
                    let resp: Response;

                    try {
                        console.info(`pulling chunk ${chunk + 1}`);
                        resp = await ApiDownloadData(fileIdHex, sessionId);
                        chunk++;
                    } catch (e) {
                        retries++;
                        if (retries > retryAmnt) {
                            await signalDone();
                            controller.error(new Error('Max retries exceeded'));
                            return;
                        }
                        console.info(`retrying after ${500 * retries}ms`);
                        await sleep(500 * retries);
                        continue;
                    }

                    if (resp.status !== 206) {
                        await signalDone();
                        controller.error(new Error(`Unexpected status: ${resp.status}`));
                        return;
                    }

                    if (resp.body === null) {
                        await signalDone();
                        controller.error(new Error('Null response body'));
                        return;
                    }

                    const reader = resp.body.getReader();
                    while (true) {
                        const {done, value} = await reader.read();
                        if (done) break;
                        controller.enqueue(value);
                        bytesRead += value.length;
                    }

                    // wait for the next pull call
                    return;
                }

                console.info("download finished");
                await signalDone();
                controller.close();
            },
        });

        const encodedFilename = encodeURIComponent(filename);
        return new Response(stream, {
            status: 200,
            headers: {
                'Content-Type': 'application/octet-stream',
                'Content-Length': String(fileSize),
                'Content-Disposition': `attachment; filename="${filename}"; filename*=UTF-8''${encodedFilename}`,
            },
        });
    })();
};

self.addEventListener('install', (event: ExtendableEvent) => {
    console.info("install");
    event.waitUntil(self.skipWaiting());
});

self.addEventListener('activate', (event: ExtendableEvent) => {
    console.info("activate");
    event.waitUntil(self.clients.claim());
});

self.addEventListener('fetch', (event: FetchEvent) => {
    const url = new URL(event.request.url);
    console.info("got url: ", url);
    if (!url.pathname.startsWith(SW_DL_PREFIX)){
        return;
    }

    const fileIdHex = url.pathname.slice(SW_DL_PREFIX.length);

    // Kind of annoying that we need to pass the password through query params,
    // but it's probably the simplest option that doesn't involve MessageChannel back and forth with the service worker.
    // We can't use regular fetch requests because we want to trigger a browser download so the user can stream the file to disk.
    const pass = url.searchParams.get('p');
    const token = (pass) ? btoa(`0:${pass}`) : null;

    event.respondWith(handleDownloadRequest(fileIdHex, token));
});
