export function fmtProgress(loaded: number, total: number, startTime: number): string {
    const percent = ((loaded / total) * 100).toFixed(1);
    const rateBps = loaded / ((Date.now() - startTime) / 1000) / 1024;
    const rateStr = rateBps > 1024 ? (rateBps / 1024).toFixed(2) + ' MB/s' : rateBps.toFixed(1) + ' KB/s';
    return `${loaded} / ${total} bytes (${percent}%) @ ${rateStr}`;
}

export function sleep(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
}
