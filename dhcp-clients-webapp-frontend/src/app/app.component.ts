import { Component } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import {DhcpClientsTableComponent } from './dhcp-clients-table/dhcp-clients-table.component'

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, DhcpClientsTableComponent],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css'
})
export class AppComponent {
  title = 'dhcp-clients-webapp-frontend';
}
