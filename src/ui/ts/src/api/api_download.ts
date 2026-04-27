import {DownloadSessionInitResponse} from './types';
import {apiFetch, authBasic, authBearer, fetchJson, fetchNone} from './util';

export const ApiDownloadInit = (fileId: string, basicToken: string | null) => {
    return fetchJson<DownloadSessionInitResponse>(`/api/dl/s/init/${fileId}`, {
        method: 'GET',
        headers: basicToken != null ? {Authorization: authBasic(basicToken)} : {},
    });
};

export const ApiDownloadData = (fileId: string, sessionId: string) => {
    return apiFetch(`/api/dl/s/data/${fileId}`, {
        method: 'GET',
        headers: {
            Authorization: authBearer(sessionId),
        },
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
