import {DownloadSessionInitResponse} from './types';
import {apiFetch, authBasic, authBearer, fetchJson, fetchNone} from './util';

export const ApiDownloadInit = (fileId: string, basicToken: string | null) => {
    return fetchJson<DownloadSessionInitResponse>(`/api/dl/s/init/${fileId}`, {
        method: 'GET',
        headers: basicToken != null ? {Authorization: authBasic(basicToken)} : {},
    });
};

export const ApiDownloadData = (fileId: string, sessionId: string, start?: number, end?: number) => {
    const headers: Record<string, string> = {
        Authorization: authBearer(sessionId),
    };

    if (start !== undefined && end !== undefined) {
        headers['Range'] = `bytes=${start}-${end}`;
    } else if (start !== undefined) {
        headers['Range'] = `bytes=${start}-`;
    }

    return apiFetch(`/api/dl/s/data/${fileId}`, {
        method: 'GET',
        headers: headers,
    });
};

export const ApiDownloadDone = (fileId: string, sessionId: string) => {
    return fetchNone(`/api/dl/s/done/${fileId}`, {
        method: 'GET',
        headers: {
            Authorization: authBearer(sessionId),
        },
    });
};
