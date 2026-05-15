export type DownloadSessionInitResponse = {
    session_id: string;
    file_size: number;
    filename: string;
    max_chunk_size: number;
};

export type UploadSessionInitResponse = {
    upload_id: string;
    max_chunk_size: number;
};

export type UploadSessionDataResponse = {
    bytes_received: number;
};

export type UploadSessionDoneResponse = {
    file_id: string;
};
