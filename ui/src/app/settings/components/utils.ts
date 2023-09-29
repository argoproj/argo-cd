export function convertExpiresInToSeconds(expiresIn: string): number {
    if (!expiresIn) {
        return 0;
    }
    const time = expiresIn.match('^([0-9]+)([smhd])$');
    const duration = parseInt(time[1], 10);
    let interval = 1;
    if (time[2] === 'm') {
        interval = 60;
    } else if (time[2] === 'h') {
        interval = 60 * 60;
    } else if (time[2] === 'd') {
        interval = 60 * 60 * 24;
    }
    return duration * interval;
}

export function validExpiresIn(expiresIn: string): boolean {
    if (!expiresIn) {
        return true;
    }
    return expiresIn.match('^([0-9]+)([smhd])$') !== null;
}
