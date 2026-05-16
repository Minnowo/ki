import {ApiDownloadData, ApiDownloadDone, ApiDownloadInit} from '../api/api_download';
import {SW_DL_PREFIX} from '../constants';
import {sleep} from '../util';

export {};

declare var self: ServiceWorkerGlobalScope;

const downloads: Map<string, { downloaded: number; total: number }> = new Map();

async function broadcastProgress() {
  const clients = await self.clients.matchAll({ includeUncontrolled: true });
  for (const client of clients) {
    client.postMessage({
      type: 'DOWNLOAD_PROGRESS',
      downloads: [...downloads]
    });
  }
}

const handleDownloadRequest = (fileIdHex: string, token: string | null): Promise<Response> => {
    return (async (): Promise<Response> => {
        console.info('beginning chunked download');

        let sessionId: string;
        let filename: string;
        let fileSize: number;
        let maxChunk: number;
        try {
            const init = await ApiDownloadInit(fileIdHex, token);
            sessionId = init.session_id;
            filename = init.filename;
            fileSize = init.file_size;
            maxChunk = init.max_chunk_size;
        } catch (e) {
            return new Response('Failed to initialize download session', {status: 400});
        }

        const signalDone = () => ApiDownloadDone(fileIdHex, sessionId).catch(() => {});


        const MIN_SIZE = 1024 * 1024;
        const MAX_SIZE = 10 * 1024 * 1024;

        const minFetchSize = Math.min(MIN_SIZE, Math.floor(maxChunk / 2));
        const retryAmnt = 10;
        let bytesRead = 0;
        let retries = 0;
        let chunk = 0;

        const queueingStrategy = new ByteLengthQueuingStrategy({
            highWaterMark: Math.min(MAX_SIZE, Math.max(MIN_SIZE, maxChunk)),
        });

        const stream = new ReadableStream<Uint8Array>(
            {
                async cancel(reason: string) {
                    console.error(`Canceled download: ${reason}`);
                    await signalDone();
                },
                async pull(controller: ReadableStreamDefaultController) {

                    let size;

                    while (true) {
                        // https://developer.mozilla.org/en-US/docs/Web/API/ReadableByteStreamController/desiredSize
                        // desiredSize == null: stream errored
                        // desiredSize == 0   : stream closed
                        // desiredSize <  0   : need to apply backpressure
                        // desiredSize >  0   : number of bytes wanted
                        size = controller.desiredSize;

                        if (size === null) {
                            console.warn('stream errored, null desired size');
                            return;
                        }

                        if (size === 0) {
                            console.warn('stream closed, 0 desired size');
                            return;
                        }

                        if (size <= minFetchSize) {
                            console.info(`waiting for backpressure.... (size: ${size})`);
                            await sleep(100);
                            continue;
                        }

                        // always want the desired size to be >= 1
                        size -= 1;

                        break;
                    }

                    while (bytesRead < fileSize) {
                        let resp: Response;

                        try {
                            // this is for the http range header, so the end-1 is normal.
                            // server will never send more than it's max chunk size, no need to worry about requesting more than that.
                            const start = bytesRead;
                            const end = bytesRead + size - 1;

                            console.info(`pulling chunk ${chunk + 1} (${start} to ${end}) (${size} bytes)`);

                            resp = await ApiDownloadData(fileIdHex, sessionId, start, end);
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
                            try {
                                const { done, value } = await reader.read();
                                if (done) {
                                    break;
                                }
                                controller.enqueue(value);
                                bytesRead += value.length;

                                downloads.set(fileIdHex, { downloaded: bytesRead, total: fileSize });
                                await broadcastProgress();

                            } catch (e) {
                                console.error(e);
                                return;
                            }
                        }

                        // wait for the next pull call
                        return;
                    }

                    console.info('download finished');
                    await signalDone();
                    controller.close();
                },
            },
            queueingStrategy
        );

        const encodedFilename = encodeURIComponent(filename);
        return new Response(stream, {
            status: 200,
            headers: {
                'Content-Security-Policy': "default-src 'none'",
                'X-Content-Security-Policy': "default-src 'none'",
                'X-WebKit-CSP': "default-src 'none'",
                'X-XSS-Protection': '1; mode=block',
                'Cross-Origin-Embedder-Policy': 'require-corp',

                'Content-Type': 'application/octet-stream; charset=utf-8',
                'Content-Length': String(fileSize),
                'Content-Disposition': `attachment; filename="${filename}"; filename*=UTF-8''${encodedFilename}`,
            },
        });
    })();
};

self.addEventListener('message', (event) => {
    event.source?.postMessage({ reply: 'pong' });
});

self.addEventListener('install', (_: ExtendableEvent) => {
    console.info('install');
    self.skipWaiting();
});

self.addEventListener('activate', (event: ExtendableEvent) => {
    console.info('activate');
    event.waitUntil(self.clients.claim());
});

self.addEventListener('fetch', (event: FetchEvent) => {
    const url = new URL(event.request.url);
    console.info('got url: ', url);

    if (!url.pathname.startsWith(SW_DL_PREFIX)) {
        return;
    }

    if (url.pathname.endsWith('/ping')) {
        return event.respondWith(new Response('pong'))
    }

    const fileIdHex = url.pathname.slice(SW_DL_PREFIX.length);

    // Kind of annoying that we need to pass the password through query params,
    // but it's probably the simplest option that doesn't involve MessageChannel back and forth with the service worker.
    // We can't use regular fetch requests because we want to trigger a browser download so the user can stream the file to disk.
    const pass = url.searchParams.get('p');
    const token = pass ? btoa(`0:${pass}`) : null;

    event.respondWith(handleDownloadRequest(fileIdHex, token));
});
