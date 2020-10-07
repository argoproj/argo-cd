import {Observable, Observer} from 'rxjs';
import {Node, Pod, PodPhase, ResourceName} from '../../../shared/models';
import {Adjectives, Animals} from './names';

const podStatusWeights = {
    Healthy: 0.7,
    OutOfSync: 0.2,
    Degraded: 0.1
};

function generateInt(min: number, max: number): number {
    return Math.floor(min + Math.random() * Math.floor(max + 1 - min));
}

function generateName(prefix: string) {
    return `${prefix}-${Adjectives[generateInt(0, Adjectives.length - 1)]}-${Animals[generateInt(0, Animals.length - 1)]}-${generateInt(100, 999)}`;
}

function generateNode(): Node {
    return {
        metadata: { name:  generateName('node') },
        status: {
            capacity: [
                {
                    name: ResourceName.ResourceCPU,
                    used: generateInt(0, 100),
                    quantity: 100
                },
                {
                    name: ResourceName.ResourceMemory,
                    used: generateInt(0, 1024),
                    quantity: 1024
                }
            ]
        }
    };
}

function generatePodPhase(weights: number[]): PodPhase {
    const sum = weights.reduce((acc, el) => acc + el, 0);
    let accumulator = 0;
    weights = weights.map(item => (accumulator = item + accumulator));
    const rand = Math.random() * sum;
    return Object.values(PodPhase)[weights.filter(el => el <= rand).length];
}

function podSort(a: Pod, b: Pod): number {
    return a.metadata.name < b.metadata.name ? -1 : 1;
}

function generatePods(n: number, nodeName: string): Pod[] {
    const pods: Pod[] = [];
    while (n) {
        pods.push({
            metadata: {name: generateName('pod').toLowerCase()},
            spec: {nodeName},
            status: {
                phase: generatePodPhase(Object.values(podStatusWeights)),
                message: ''
            }
        });
        n--;
    }
    pods.sort(podSort);
    return pods;
}

function randomAdjustmentOf(x: number, max: number, maxPercent: number): number {
    const sign = generateInt(0, 1) ? 1 : -1;
    const p = (generateInt(0, 100) / 100) * (maxPercent / 100);
    let res = x * (1 + sign * p);
    if (res > max) {
        res = x * (1 - sign * p);
        return res < 0 ? x : res;
    }
    return res;
}

export function GetNodes(x: number, nodes?: Node[]): Observable<Node[]> {
    return Observable.create((observer: Observer<Node[]>) => {
        const interval = setInterval(() => {
            nodes = nodes || [];
            if (nodes.length < x) {
                let n = x - nodes.length;
                while (n) {
                    nodes.push(generateNode());
                    n--;
                }
            } else {
                nodes = nodes.map(n => {
                    const u = {...n};
                    u.cpu.cur = Math.round(randomAdjustmentOf(u.cpu.cur, u.cpu.max, 3));
                    u.mem.cur = Math.round(randomAdjustmentOf(u.mem.cur, u.mem.max, 3));
                    u.pods = u.pods
                        .map(p => {
                            const up = {...p};
                            if (generateInt(0, 100) < 5) {
                                up.status = generatePodPhase(Object.values(podStatusWeights));
                            }
                            return up;
                        })
                        .sort(podSort);
                    return u;
                });
            }
            observer.next(nodes);
        }, 1000);
        return () => clearInterval(interval);
    });
}
