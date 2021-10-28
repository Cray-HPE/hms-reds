# River Endpoint Discovery Service

The River Endpoint Discovery Service is responsible for endpoint discovery, initialization and geolocation of hardware in River cabinets.

## The Tasks

* Endpoint Discovery -- The process of locating a previously-unknown Redfish endpoint on the network
* Initialization -- The process of configuring a previously-unknown Redfish endpoint to work within the Cray.  This may involve setting credentials or running other configuration tasks
* Geolocation -- The process of determining the physical location of the new Redfish endpoint and assigning it an xname

## How it works

The two major components of REDS are a daemon (contained in a kubernetes pod) and a boot image.  These two cooperate to perform many of the tasks required of REDS.

When a piece of hardware unknown to the Boot Script Service attempts to boot, it is given a boot script that causes it to boot the REDS image.  The REDS image then performs initialization on the node -- it sets the BMC credentials and ensures the BMC is configured to DHCP for its IP address.  Next the image gathers information about the node, including BMC MAC addresses.  Finally, the image bundles up the BMC crednetials and information about the node and transmits it to the daemon.

In parallel, the daemon monitors the mac address tables of the switches on the hardware management network.  When it locates a previously-unknown mac address, it makes note of it and the port it is conencted to.

When the daemon recieves data from an instance of the boot image, it looks at the mac addresses and attempts to correlate them with the ones from the switches.  When it is able to do so, it uses the port to determine which xname should be assigned ot the node.  This assignment is done via a user-configured mapping from port to xname (a.k.a.: the "REDS Mapping file").

Once an xname is assigned, all required information has been gathered and the endpoint's IP, credentials and xname are tranferred to HSM.

## Configuration

REDS takes no direct configuration, except for the location of the mapping file.  The location of the mapping file may be altered bby changing the value of `cray_reds_mapping_file`.

The mapping file itself should be built using the [ccd-reader utility](https://stash.us.cray.com/projects/HMS/repos/hms-ccd-reader/browse).  If somehow you end up having to hand-build a configuration file, it looks like this, excluding `// comments`:

```
{
    "version":1, // Always 1
    "switches":[
        {
            "id":"x3000c0w38", // xname for the switch
            "address":"10.4.255.254", // IP address of the switch on the hardware management network
            "snmpUser":"root", // SNMP username for the switch
            "snmpAuthPassword":"********", // SNMP authentication password
            "snmpAuthProtocol":"MD5", // SNMP authentication protocol. MD5 or SHA1
            "snmpPrivPassword":"********", // SNMP Privacy password
            "snmpPrivProtocol":"DES", // SNMP privacy protocol. DES or AES
            "model":"Dell S3048-ON", // Name of the switch model
            "ports":[ // A list of ports on the hardware management network with compute node BMCs attached
                {
                    "id":0, // A unique integer
                    "ifName":"GigabitEthernet 1/46", // FULL name of the port as shown in the switch's management console.
                    "peerID":"x0c0s25b0" // xname of the hardware attached ot this port
                },
                ... // More ports
            ]
        },
        ... // More switches
    ]
}
```

## REDS CT Testing

This repository builds and publishes hms-reds-ct-test RPMs along with the service itself containing tests that verify REDS on the
NCNs of live Shasta systems. The tests require the hms-ct-test-base RPM to also be installed on the NCNs in order to execute.
The version of the test RPM installed on the NCNs should always match the version of REDS deployed on the system.

## More Information

* [REDS design document](https://connect.us.cray.com/confluence/pages/viewpage.action?pageId=110180596)
* [REDS/MEDS/IDEALS interaction](https://connect.us.cray.com/confluence/pages/viewpage.action?pageId=136352226)

## Future Work
TBD.