import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { WebsocketService } from '../websocket.service';

@Component({
  selector: 'app-dhcp-clients-table',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './dhcp-clients-table.component.html',
  styleUrl: './dhcp-clients-table.component.css'
})
export class DhcpClientsTableComponent {
  ipMacList: { ip: string, mac: string }[] = [];

  constructor(private websocketService: WebsocketService) { }

  ngOnInit(): void {
    this.websocketService.getData().subscribe(data => {
      this.ipMacList.push(data);
    });
  }
}
