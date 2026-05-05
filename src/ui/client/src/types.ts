export {};

declare global {
    interface SaveFilePickerOptions {
        suggestedName?: string | undefined;
    }

    interface Window {
        InitUpload: (mountId: string, formId: string, maxUploadSize: number, chunkSize: number) => void;
        InitDownload: (mountId: string, fileIdHex: string, filename: string, hasPassword: boolean) => void;
        showSaveFilePicker?: (options?: SaveFilePickerOptions) => Promise<FileSystemFileHandle>;
    }
}
