#!/bin/bash

OUTPUT_DIR=./external-libs
mkdir -p $OUTPUT_DIR

# copy JS
cp node_modules/jquery/dist/jquery.slim.min.js $OUTPUT_DIR
cp node_modules/jszip/dist/jszip.min.js $OUTPUT_DIR

cp node_modules/datatables.net/js/dataTables.min.js $OUTPUT_DIR
cp node_modules/datatables.net-dt/js/dataTables.dataTables.min.js $OUTPUT_DIR

cp node_modules/datatables.net-responsive/js/dataTables.responsive.min.js $OUTPUT_DIR
cp node_modules/datatables.net-responsive-dt/js/responsive.dataTables.min.js $OUTPUT_DIR

cp node_modules/datatables.net-select/js/dataTables.select.min.js $OUTPUT_DIR
cp node_modules/datatables.net-select-dt/js/select.dataTables.min.js $OUTPUT_DIR

cp node_modules/datatables.net-buttons/js/dataTables.buttons.min.js $OUTPUT_DIR
cp node_modules/datatables.net-buttons-dt/js/buttons.dataTables.min.js $OUTPUT_DIR
cp node_modules/datatables.net-buttons/js/buttons.colVis.min.js $OUTPUT_DIR
cp node_modules/datatables.net-buttons/js/buttons.html5.min.js $OUTPUT_DIR
cp node_modules/datatables.net-buttons/js/buttons.print.min.js $OUTPUT_DIR

cp node_modules/datatables.net-plugins/sorting/ip-address.min.js $OUTPUT_DIR


# copy CSS

cp node_modules/datatables.net-dt/css/dataTables.dataTables.min.css $OUTPUT_DIR
cp node_modules/datatables.net-responsive-dt/css/responsive.dataTables.min.css $OUTPUT_DIR

cp node_modules/datatables.net-select-dt/css/select.dataTables.min.css $OUTPUT_DIR
