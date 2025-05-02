#!/bin/bash

OUTPUT_DIR=./external-libs
mkdir -p $OUTPUT_DIR

# copy JS
cp node_modules/jquery/dist/jquery.slim.min.js $OUTPUT_DIR
cp node_modules/datatables.net/js/dataTables.min.js $OUTPUT_DIR
cp node_modules/datatables.net-dt/js/dataTables.dataTables.min.js $OUTPUT_DIR
cp node_modules/datatables.net-responsive/js/dataTables.responsive.min.js $OUTPUT_DIR
cp node_modules/datatables.net-responsive-dt/js/responsive.dataTables.min.js $OUTPUT_DIR
cp node_modules/datatables.net-plugins/sorting/ip-address.min.js $OUTPUT_DIR

cp node_modules/datatables.net-dt/css/dataTables.dataTables.min.css $OUTPUT_DIR
cp node_modules/datatables.net-responsive-dt/css/responsive.dataTables.min.css $OUTPUT_DIR