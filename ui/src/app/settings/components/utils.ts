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

// Utility function to recursively trim whitespaces
export function trimStringProperties(item: any): any {
    if (typeof item === 'string') {
        return item.trim();
    }

    if (Array.isArray(item)) {
        return item.map(element => trimStringProperties(element));
    } else if (typeof item === 'object' && item !== null) {
        const result: {[key: string]: any} = {};
        for (const key in item) {
            if (item.hasOwnProperty(key)) {
                const value = trimStringProperties(item[key]);
                if (!(typeof value === 'object' && !Array.isArray(value))) {
                    result[key] = value;
                }
            }
        }
        return result;
    }
    return item;
}
