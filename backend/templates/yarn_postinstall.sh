#!/bin/bash
mkdir -p libs

# copy JS
cp node_modules/jquery/dist/jquery.slim.min.js libs/
cp node_modules/datatables.net/js/dataTables.min.js libs/
cp node_modules/datatables.net-dt/js/dataTables.dataTables.min.js libs/
cp node_modules/datatables.net-responsive/js/dataTables.responsive.min.js libs/
cp node_modules/datatables.net-responsive-dt/js/responsive.dataTables.min.js libs/
cp node_modules/datatables.net-plugins/sorting/ip-address.min.js libs/

cp node_modules/datatables.net-dt/css/dataTables.dataTables.min.css libs/
cp node_modules/datatables.net-responsive-dt/css/responsive.dataTables.min.css libs/