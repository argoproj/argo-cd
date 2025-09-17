import * as jsYaml from 'js-yaml';

// Constants for performance optimization
export const YAML_SIZE_LIMITS = {
    // Maximum size for YAML dump before truncation (in characters)
    MAX_YAML_SIZE: 100000, // ~100KB
    // Maximum number of lines to show in diff
    MAX_DIFF_LINES: 1000,
    // Maximum context lines around changes
    MAX_CONTEXT_LINES: 50,
    // Size threshold to show performance warning
    WARNING_THRESHOLD: 50000 // ~50KB
};

export interface YamlPerformanceInfo {
    size: number;
    lineCount: number;
    isLarge: boolean;
    needsTruncation: boolean;
    warningMessage?: string;
}

/**
 * Analyzes YAML content for performance characteristics
 */
export function analyzeYamlPerformance(content: string): YamlPerformanceInfo {
    const size = content.length;
    const lineCount = content.split('\n').length;
    const isLarge = size > YAML_SIZE_LIMITS.WARNING_THRESHOLD;
    const needsTruncation = size > YAML_SIZE_LIMITS.MAX_YAML_SIZE;

    let warningMessage: string | undefined;
    if (needsTruncation) {
        warningMessage = `Resource is very large (${Math.round(size / 1000)}KB, ${lineCount} lines). Content has been truncated for performance.`;
    } else if (isLarge) {
        warningMessage = `Resource is large (${Math.round(size / 1000)}KB, ${lineCount} lines). Performance may be affected.`;
    }

    return {
        size,
        lineCount,
        isLarge,
        needsTruncation,
        warningMessage
    };
}

/**
 * Safely dumps YAML with size limits and performance optimizations
 */
export function safeYamlDump(obj: any, options: jsYaml.DumpOptions = {}): {yaml: string; info: YamlPerformanceInfo} {
    const defaultOptions: jsYaml.DumpOptions = {
        indent: 2,
        lineWidth: -1, // Disable line wrapping for better performance
        noRefs: true, // Disable references for better performance
        ...options
    };

    let yaml = jsYaml.dump(obj, defaultOptions);
    const info = analyzeYamlPerformance(yaml);

    if (info.needsTruncation) {
        // Truncate the YAML content
        const lines = yaml.split('\n');
        const truncatedLines = lines.slice(0, YAML_SIZE_LIMITS.MAX_DIFF_LINES);
        yaml = truncatedLines.join('\n') + '\n# ... (content truncated for performance)';
    }

    return {yaml, info};
}

/**
 * Truncates diff content to prevent browser freezing
 */
export function truncateDiffContent(diffText: string): {content: string; wasTruncated: boolean} {
    const lines = diffText.split('\n');
    const wasTruncated = lines.length > YAML_SIZE_LIMITS.MAX_DIFF_LINES;

    if (wasTruncated) {
        const truncatedLines = lines.slice(0, YAML_SIZE_LIMITS.MAX_DIFF_LINES);
        truncatedLines.push('# ... (diff truncated for performance)');
        return {content: truncatedLines.join('\n'), wasTruncated};
    }

    return {content: diffText, wasTruncated};
}

/**
 * Optimizes diff context for large files
 */
export function optimizeDiffContext(context: number, contentSize: number): number {
    if (contentSize > YAML_SIZE_LIMITS.WARNING_THRESHOLD) {
        // For large content, limit context to prevent performance issues
        return Math.min(context, YAML_SIZE_LIMITS.MAX_CONTEXT_LINES);
    }
    return context;
}
