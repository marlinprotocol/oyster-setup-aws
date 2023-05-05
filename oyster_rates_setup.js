#!/usr/bin/env node

// Script to setup rates file for oyster.

// Pre : npm install @aws-sdk/client-pricing && npm install @aws-sdk/client-ec2
// Pre : make it executable : chmod +x oyster_rates_setup.js

// Rates file written in the /home/user/.marlin folder

// To use for or exclude certain regions and instance types modify the run function as needed

const fs = require("fs");
const { PricingClient, GetProductsCommand } = require("@aws-sdk/client-pricing");
const { EC2Client, DescribeInstanceTypesCommand } = require('@aws-sdk/client-ec2');

const ec2Client = new EC2Client();
const pricingClient = new PricingClient();

// Function to get all the instance types using the nitro hypervisor i.e. supporting secure enclaves with vcpus > 1
async function getAllInstanceTypesWithNitro() {
    const input = {};
    const command = new DescribeInstanceTypesCommand(input);

    try {
        const response = await ec2Client.send(command);

        const instanceTypes = response.InstanceTypes.filter((instanceType) => {
            return (instanceType.Hypervisor === 'nitro') && (instanceType.VCpuInfo.DefaultVCpus >= 2);
        }).map((instanceType) => {
            return {
                instanceType: instanceType.InstanceType,
                vCpus: instanceType.VCpuInfo.DefaultVCpus
            };
        });

        return instanceTypes.map(i => i.instanceType);
    } catch (error) {
        console.error(error);
        return [];
    }
}

// Function to get the price of DedicatedUsage of an instance type in all supported regions 
async function getEc2Prices(instanceType) {
    const params = {
        ServiceCode: 'AmazonEC2',
        Filters: [
            {
                Type: 'TERM_MATCH',
                Field: 'instanceType',
                Value: instanceType
            },
            {
                Type: 'TERM_MATCH',
                Field: 'operatingSystem',
                Value: 'Linux'
            },
            {
                Type: 'TERM_MATCH',
                Field: 'preInstalledSw',
                Value: 'NA'
            },
        ]
    };

    const command = new GetProductsCommand(params);

    try {
        const response = await pricingClient.send(command);

        const products = response.PriceList.map((product) => JSON.parse(product));

        const productsFiltered = products.filter(i => {
            return i.product.attributes.usagetype.includes('DedicatedUsage')
        }).map((instance) => ({
            region: instance.product.attributes.regionCode,
            instance: instance.product.attributes.instanceType,
            min_rate: parseInt(parseFloat(instance.terms.OnDemand[Object.keys(instance.terms.OnDemand)[0]]
                .priceDimensions[Object.keys(instance.terms.OnDemand[Object.keys(instance.terms.OnDemand)[0]]
                    .priceDimensions)[0]].pricePerUnit.USD).toFixed(6) * 1e6)
        }))


        // console.log(util.inspect(productsFiltered, false, null, true))

        const list = productsFiltered.filter(i => { return i.min_rate > 0 })

        return list;
    } catch (error) {
        console.error(error);
    }
}

async function run() {

    const ec2InstanceTypes = await getAllInstanceTypesWithNitro();

    const excludedRegions = [];
    const excludedInstances = [];
    const selectInstanceFamiliesOnly = false;
    const selectInstanceFamilies = ['c6a.',];
    const selectRegionsOnly = false;
    const selectRegions = [];

    let products = [];
    for (let i = 0; i < ec2InstanceTypes.length; i++) {
        const res = await getEc2Prices(ec2InstanceTypes[i]);
        products.push(...res);
    }

    // Change into [{region,[{instance,min_rate}]}] format
    const result = products.reduce((newProds, curr) => {
        if (excludedInstances.includes(curr.instance) || excludedRegions.includes(curr.region)) return newProds;
        else if (selectInstanceFamiliesOnly && !selectInstanceFamilies.some(prefix => curr.instance.startsWith(prefix))) return newProds;
        else if (selectRegionsOnly && !selectRegions.includes(curr.region)) return newProds;

        const found = newProds.find(el => el.region === curr.region);

        if (found) {
            found.rate_cards.push({ instance: curr.instance, min_rate: curr.min_rate });
        } else {
            newProds.push({
                region: curr.region,
                rate_cards: [{ instance: curr.instance, min_rate: curr.min_rate },]
            });
        }
        return newProds;
    }, []);
    // console.log(util.inspect(result, false, null, true))

    // Write to .marlin folder
    const data = JSON.stringify(result);
    fs.writeFile('/home/' + require("os").userInfo().username + '/.marlin/rates.json', data, (error) => {
        if (error) {
            console.error(error);

            throw error;
        }

        console.log('rates.json written correctly');
    });
}

run();
