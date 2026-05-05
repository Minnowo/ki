export const authBearer = (token: string) => {
    return `Bearer ${token}`;
};
export const authBasic = (token: string) => {
    return `Basic ${token}`;
};

export const apiFetch = (path: string, args?: RequestInit) => {
    const headers = new Headers(args?.headers);

    headers.set('X-Requested-With', 'js-form');

    return fetch(path, {...args, headers});
};

export const fetchJson = async <T>(path: string, args?: RequestInit): Promise<T> => {
    return apiFetch(path, args).then((res) => res.json());
};

export const fetchNone = async (path: string, args?: RequestInit): Promise<void> => {
    return apiFetch(path, args).then(() => undefined);
};
