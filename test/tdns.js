import dns from 'k6/x/dns';
import {sleep} from 'k6';
import {SharedArray} from 'k6/data';
import {vu} from 'k6/execution';
import {randomIntBetween} from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';


const test = new SharedArray('domains', function () {
    return JSON.parse(open('./data.json')).test;
});

export const options = {
    scenarios: {
        login: {
            executor: 'per-vu-iterations',
            vus: test.length,
            iterations: 40,
            maxDuration: '40s',
        },
    },
};


export default async function () {
    // Request the IP address of k6.io from the selected namerserver A records.
    let domain = test[vu.idInTest - 1].domains[randomIntBetween(0, test[vu.idInTest - 1].domains.length - 1)]
    //console.log(`Resolving domain: ${domain}`);
    const resolvedIP = await dns.resolve(domain, 'A', '127.0.0.1:8053');
    console.log(`${domain} IP as resolved against the tdns nameserver: ${resolvedIP}`);
    sleep(randomIntBetween(1, 3));
}






